// Layout parsing for themes (issue #96, ADR 0002 §3). A theme may ship one XML
// file per view in layouts/<view>.xml, arranging the catalog's blocks in its
// regions and rows. Only the core reads these files; the UI gets the validated
// structs over RPC. Unknown names are ignored with a warning, never an error,
// and a layout that leaves out a required block still gets it, in its built-in
// region, with a warning.
package themes

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"platypus/internal/types"
)

// supportsLayouts is the theme level that turns shipped layouts on. Like
// tokens and variants, layouts are parsed always (so the picker shows their
// warnings) and applied only when the theme declares the level.
const supportsLayouts = "layouts"

// layoutParser validates one layouts/<view>.xml against the catalog, keeping
// the warnings for the theme list. seen records the blocks placed, for the
// required check.
type layoutParser struct {
	view     string
	warnings []string
	seen     map[string]bool
	// broken is set when the document stops mid-way: the layout is then
	// discarded, since a half-read tree is worse than the built-in one.
	broken bool
}

func (p *layoutParser) warn(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

// parseLayout reads and validates one layout file. fileView is the view from
// the file name; the view attribute must agree with it. A nil layout means the
// theme ships nothing usable for the view: the UI then uses its built-in
// arrangement.
func parseLayout(fileView string, src []byte) (*types.ThemeLayout, []string) {
	p := &layoutParser{view: fileView, seen: map[string]bool{}}
	return p.parse(src), p.warnings
}

func (p *layoutParser) parse(src []byte) *types.ThemeLayout {
	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.Strict = true
	root, err := startElement(dec)
	if err != nil {
		p.warn("layout for view %q can't be read: %v, using the built-in layout", p.view, err)
		return nil
	}
	if root.Name.Local != "layout" {
		p.warn("layout for view %q starts with <%s>, want <layout>, using the built-in layout", p.view, root.Name.Local)
		return nil
	}
	views := layoutViews()
	if !views[p.view] {
		p.warn("unknown layout view %q, ignored", p.view)
		p.skipRest(dec)
		return nil
	}
	var viewAttr, catalog string
	for _, a := range root.Attr {
		switch a.Name.Local {
		case "view":
			viewAttr = a.Value
		case "catalog":
			catalog = a.Value
		default:
			p.warn("unknown layout attribute %q, ignored", a.Name.Local)
		}
	}
	if viewAttr != "" && viewAttr != p.view {
		p.warn("layout view %q doesn't match its file %q, reading it as %q", viewAttr, p.view, p.view)
	}
	if catalog == "" {
		catalog = DefaultThemeCatalog
	}
	targetMajor, _, validCatalog := catalogVersion(catalog)
	currentMajor, _, _ := catalogVersion(CatalogVersion)
	if !validCatalog {
		p.warn("layout catalog must be a major.minor version, assuming %s", DefaultThemeCatalog)
	} else if targetMajor > currentMajor {
		p.warn("layout needs catalog %s, this Platypus reads catalog %s, using the built-in layout", catalog, CatalogVersion)
		p.skipRest(dec)
		return nil
	}
	out := &types.ThemeLayout{View: p.view}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.warn("layout for view %q can't be read: %v, using the built-in layout", p.view, err)
			return nil
		}
		end, ok := tok.(xml.EndElement)
		if ok && end.Name.Local == "layout" {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "region" {
			p.warn("unknown layout element <%s> in view %q, ignored", start.Name.Local, p.view)
			skipElement(dec, start)
			continue
		}
		if region := p.region(dec, start); region != nil {
			out.Regions = append(out.Regions, *region)
		}
		if p.broken {
			return nil
		}
	}
	p.restoreRequired(out)
	return out
}

// region reads one <region>: its name must be one the catalog knows for the
// view, and its children are <block> and <row> in the order written.
func (p *layoutParser) region(dec *xml.Decoder, start xml.StartElement) *types.ThemeLayoutRegion {
	var name string
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "name":
			name = a.Value
		default:
			p.warn("unknown region attribute %q in view %q, ignored", a.Name.Local, p.view)
		}
	}
	known := layoutRegions(p.view)
	if !known[name] {
		if name == "" {
			p.warn("a region in view %q has no name, ignored", p.view)
		} else {
			p.warn("unknown region %q in view %q, ignored", name, p.view)
		}
		skipElement(dec, start)
		return nil
	}
	region := &types.ThemeLayoutRegion{Name: name}
	for {
		tok, err := dec.Token()
		if err != nil {
			p.warn("layout for view %q can't be read: %v, using the built-in layout", p.view, err)
			p.broken = true
			return nil
		}
		if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "region" {
			return region
		}
		child, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch child.Name.Local {
		case "block":
			if node := p.block(dec, child); node != nil {
				region.Nodes = append(region.Nodes, *node)
			}
		case "row":
			if node := p.row(dec, child); node != nil {
				region.Nodes = append(region.Nodes, *node)
			}
		default:
			p.warn("unknown layout element <%s> in region %q, ignored", child.Name.Local, name)
			skipElement(dec, child)
		}
	}
}

// row reads one <row>: blocks side by side. A row inside a row makes no
// sense, so a nested row is ignored with a warning.
func (p *layoutParser) row(dec *xml.Decoder, start xml.StartElement) *types.ThemeLayoutNode {
	for _, a := range start.Attr {
		p.warn("unknown row attribute %q in view %q, ignored", a.Name.Local, p.view)
	}
	node := &types.ThemeLayoutNode{}
	for {
		tok, err := dec.Token()
		if err != nil {
			p.warn("layout for view %q can't be read: %v, using the built-in layout", p.view, err)
			p.broken = true
			return nil
		}
		if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "row" {
			return node
		}
		child, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if child.Name.Local != "block" {
			p.warn("unknown layout element <%s> in a row, ignored", child.Name.Local)
			skipElement(dec, child)
			continue
		}
		if block := p.block(dec, child); block != nil {
			node.Row = append(node.Row, *block)
		}
	}
}

// block reads one <block>: a name the catalog knows for the view, plus the
// attributes the catalog declares for it (e.g. limit).
func (p *layoutParser) block(dec *xml.Decoder, start xml.StartElement) *types.ThemeLayoutNode {
	var name string
	attrs := map[string]string{}
	for _, a := range start.Attr {
		if a.Name.Local == "name" {
			name = a.Value
			continue
		}
		attrs[a.Name.Local] = a.Value
	}
	skipElement(dec, start)
	entry, ok := layoutBlock(p.view, name)
	if !ok {
		if name == "" {
			p.warn("a block in view %q has no name, ignored", p.view)
		} else {
			p.warn("unknown block %q in view %q, ignored", name, p.view)
		}
		return nil
	}
	if p.seen[name] {
		p.warn("duplicate block %q in view %q, ignored", name, p.view)
		return nil
	}
	node := &types.ThemeLayoutNode{Block: name}
	for _, key := range slices.Sorted(maps.Keys(attrs)) {
		decl, ok := findBlockAttr(entry, key)
		if !ok {
			p.warn("unknown attribute %q on block %q, ignored", key, name)
			continue
		}
		value, ok := checkBlockAttr(decl, attrs[key])
		if !ok {
			p.warn("block %q: %s %q is not valid, ignored", name, key, attrs[key])
			continue
		}
		if node.Attrs == nil {
			node.Attrs = map[string]string{}
		}
		node.Attrs[key] = value
	}
	p.seen[name] = true
	return node
}

// findBlockAttr returns the catalog's declaration for one block attribute.
func findBlockAttr(entry types.ThemeCatalogEntry, key string) (types.ThemeCatalogAttr, bool) {
	for _, a := range entry.Attrs {
		if a.Name == key {
			return a, true
		}
	}
	return types.ThemeCatalogAttr{}, false
}

// checkBlockAttr validates one declared block attribute and returns its
// canonical form. int attributes take a whole number in Min..Max (Max 0 means
// no maximum); enum attributes take one of Values.
func checkBlockAttr(attr types.ThemeCatalogAttr, value string) (string, bool) {
	switch attr.Type {
	case "int":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < attr.Min || (attr.Max != 0 && n > attr.Max) {
			return "", false
		}
		return strconv.Itoa(n), true
	case "enum":
		if !slices.Contains(attr.Values, value) {
			return "", false
		}
		return value, true
	default:
		return value, true
	}
}

// restoreRequired appends every required block the layout leaves out, in its
// built-in region, with a warning. A capability surface a layout hides would
// make a working feature invisible, so the app puts it back.
func (p *layoutParser) restoreRequired(out *types.ThemeLayout) {
	for _, entry := range Catalog {
		if entry.Kind != "block" || !entry.Required {
			continue
		}
		view, name, ok := splitBlockEntry(entry.Entry)
		if !ok || view != p.view || p.seen[name] {
			continue
		}
		p.warn("layout for view %q leaves out required block %q, showing it in %q", p.view, name, entry.Region)
		node := types.ThemeLayoutNode{Block: name}
		for i := range out.Regions {
			if out.Regions[i].Name == entry.Region {
				out.Regions[i].Nodes = append(out.Regions[i].Nodes, node)
				p.seen[name] = true
				break
			}
		}
		if !p.seen[name] {
			out.Regions = append(out.Regions, types.ThemeLayoutRegion{Name: entry.Region, Nodes: []types.ThemeLayoutNode{node}})
			p.seen[name] = true
		}
	}
}

// layoutViews are the views the catalog knows regions for: the only views a
// layout file can target.
func layoutViews() map[string]bool {
	views := map[string]bool{}
	for _, entry := range Catalog {
		if entry.Kind != "region" {
			continue
		}
		if view, _, ok := splitBlockEntry(entry.Entry); ok {
			views[view] = true
		}
	}
	return views
}

// layoutRegions are the region names the catalog knows for a view.
func layoutRegions(view string) map[string]bool {
	regions := map[string]bool{}
	for _, entry := range Catalog {
		if entry.Kind != "region" {
			continue
		}
		if v, name, ok := splitBlockEntry(entry.Entry); ok && v == view {
			regions[name] = true
		}
	}
	return regions
}

// layoutBlock returns the catalog entry for a block in a view.
func layoutBlock(view, name string) (types.ThemeCatalogEntry, bool) {
	for _, entry := range Catalog {
		if entry.Kind != "block" {
			continue
		}
		if v, n, ok := splitBlockEntry(entry.Entry); ok && v == view && n == name {
			return entry, true
		}
	}
	return types.ThemeCatalogEntry{}, false
}

// splitBlockEntry cuts a "view.name" layout entry into its view and name.
func splitBlockEntry(entry string) (view, name string, ok bool) {
	view, name, found := strings.Cut(entry, ".")
	return view, name, found && view != "" && name != ""
}

// startElement reads the first element of a document, skipping directives.
func startElement(dec *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start, nil
		}
	}
}

// skipElement skips to the end of the element that just started. It fails
// only on a malformed document, which the caller's next Token reports.
func skipElement(dec *xml.Decoder, start xml.StartElement) {
	_ = dec.Skip()
}

// skipRest drains a document the parser has given up on, so callers stay
// past its end.
func (p *layoutParser) skipRest(dec *xml.Decoder) {
	for {
		if _, err := dec.Token(); err != nil {
			return
		}
	}
}

// LayoutXSD returns the editor schema generated from the catalog: the views a
// layout can target, every region and block name, and the union of the
// per-block attributes. It stays as the compatibility schema at
// internal/themes/layout.xsd: editors that point at it accept any catalog
// name, while the core validator still rejects a block in the wrong view.
// Per-view placement lives in LayoutXSDForView (issue #110).
func LayoutXSD() string {
	views := map[string]bool{}
	regions := map[string]bool{}
	blocks := map[string]bool{}
	attrs := map[string]types.ThemeCatalogAttr{}
	for _, entry := range Catalog {
		view, name, ok := splitBlockEntry(entry.Entry)
		if !ok {
			continue
		}
		switch entry.Kind {
		case "region":
			views[view] = true
			regions[name] = true
		case "block":
			blocks[name] = true
			for _, a := range entry.Attrs {
				if _, exists := attrs[a.Name]; !exists {
					attrs[a.Name] = a
				}
			}
		}
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!-- Generated from the theme catalog: do not edit. Refresh with\n")
	b.WriteString("     go test ./internal/themes -run TestLayoutXSDCurrent -update. -->\n")
	b.WriteString("<!-- Compatibility schema: the union of every view. For precise\n")
	b.WriteString("     per-view checking use layouts/<view>.xsd (LayoutXSDForView). -->\n")
	b.WriteString("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n")
	b.WriteString("  <xs:element name=\"layout\">\n")
	b.WriteString("    <xs:complexType>\n")
	b.WriteString("      <xs:sequence>\n")
	b.WriteString("        <xs:element name=\"region\" maxOccurs=\"unbounded\">\n")
	b.WriteString("          <xs:complexType>\n")
	b.WriteString("            <xs:choice maxOccurs=\"unbounded\">\n")
	b.WriteString("              <xs:element name=\"block\" type=\"blockType\" />\n")
	b.WriteString("              <xs:element name=\"row\">\n")
	b.WriteString("                <xs:complexType>\n")
	b.WriteString("                  <xs:sequence>\n")
	b.WriteString("                    <xs:element name=\"block\" type=\"blockType\" maxOccurs=\"unbounded\" />\n")
	b.WriteString("                  </xs:sequence>\n")
	b.WriteString("                </xs:complexType>\n")
	b.WriteString("              </xs:element>\n")
	b.WriteString("            </xs:choice>\n")
	b.WriteString("            <xs:attribute name=\"name\" use=\"required\">\n")
	xsdEnum(&b, regions, 14)
	b.WriteString("            </xs:attribute>\n")
	b.WriteString("          </xs:complexType>\n")
	b.WriteString("        </xs:element>\n")
	b.WriteString("      </xs:sequence>\n")
	b.WriteString("      <xs:attribute name=\"view\" use=\"required\">\n")
	xsdEnum(&b, views, 8)
	b.WriteString("      </xs:attribute>\n")
	b.WriteString("      <xs:attribute name=\"catalog\" type=\"xs:string\" />\n")
	b.WriteString("    </xs:complexType>\n")
	b.WriteString("  </xs:element>\n")
	b.WriteString("  <xs:complexType name=\"blockType\">\n")
	b.WriteString("    <xs:attribute name=\"name\" use=\"required\">\n")
	xsdEnum(&b, blocks, 6)
	b.WriteString("    </xs:attribute>\n")
	for _, name := range slices.Sorted(maps.Keys(attrs)) {
		xsdAttr(&b, attrs[name], "    ")
	}
	b.WriteString("  </xs:complexType>\n")
	b.WriteString("</xs:schema>\n")
	return b.String()
}

// LayoutXSDForView returns the strict editor schema for one view (issue
// #110): only its regions, blocks and attribute types. It is plain XSD 1.0
// so common editors validate it: the VS Code XML extension (LemMinX) and
// IntelliJ support XSD 1.0 well and XSD 1.1 xs:alternative poorly, so one
// file per view beats a single file switching types on the view attribute.
// Files live at internal/themes/layouts/<view>.xsd; layout.xsd stays as the
// union for compatibility.
func LayoutXSDForView(view string) string {
	regions := map[string]bool{}
	blocks := map[string]bool{}
	attrs := map[string]types.ThemeCatalogAttr{}
	for _, entry := range Catalog {
		v, name, ok := splitBlockEntry(entry.Entry)
		if !ok || v != view {
			continue
		}
		switch entry.Kind {
		case "region":
			regions[name] = true
		case "block":
			blocks[name] = true
			for _, a := range entry.Attrs {
				if _, exists := attrs[a.Name]; !exists {
					attrs[a.Name] = a
				}
			}
		}
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!-- Generated from the theme catalog: do not edit. Refresh with\n")
	b.WriteString("     go test ./internal/themes -run TestLayoutXSDCurrent -update. -->\n")
	fmt.Fprintf(&b, "<!-- Strict schema for layouts/%s.xml: only this view's regions and blocks. -->\n", view)
	b.WriteString("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n")
	b.WriteString("  <xs:element name=\"layout\">\n")
	b.WriteString("    <xs:complexType>\n")
	b.WriteString("      <xs:sequence>\n")
	b.WriteString("        <xs:element name=\"region\" maxOccurs=\"unbounded\">\n")
	b.WriteString("          <xs:complexType>\n")
	b.WriteString("            <xs:choice maxOccurs=\"unbounded\">\n")
	b.WriteString("              <xs:element name=\"block\" type=\"blockType\" />\n")
	b.WriteString("              <xs:element name=\"row\">\n")
	b.WriteString("                <xs:complexType>\n")
	b.WriteString("                  <xs:sequence>\n")
	b.WriteString("                    <xs:element name=\"block\" type=\"blockType\" maxOccurs=\"unbounded\" />\n")
	b.WriteString("                  </xs:sequence>\n")
	b.WriteString("                </xs:complexType>\n")
	b.WriteString("              </xs:element>\n")
	b.WriteString("            </xs:choice>\n")
	b.WriteString("            <xs:attribute name=\"name\" use=\"required\">\n")
	xsdEnum(&b, regions, 14)
	b.WriteString("            </xs:attribute>\n")
	b.WriteString("          </xs:complexType>\n")
	b.WriteString("        </xs:element>\n")
	b.WriteString("      </xs:sequence>\n")
	fmt.Fprintf(&b, "      <xs:attribute name=\"view\" use=\"required\">\n        <xs:simpleType>\n          <xs:restriction base=\"xs:string\">\n            <xs:enumeration value=\"%s\" />\n          </xs:restriction>\n        </xs:simpleType>\n      </xs:attribute>\n", view)
	b.WriteString("      <xs:attribute name=\"catalog\" type=\"xs:string\" />\n")
	b.WriteString("    </xs:complexType>\n")
	b.WriteString("  </xs:element>\n")
	b.WriteString("  <xs:complexType name=\"blockType\">\n")
	b.WriteString("    <xs:attribute name=\"name\" use=\"required\">\n")
	xsdEnum(&b, blocks, 6)
	b.WriteString("    </xs:attribute>\n")
	for _, name := range slices.Sorted(maps.Keys(attrs)) {
		xsdAttr(&b, attrs[name], "    ")
	}
	b.WriteString("  </xs:complexType>\n")
	b.WriteString("</xs:schema>\n")
	return b.String()
}

// LayoutXSDViews are the views with a per-view schema file.
func LayoutXSDViews() []string {
	views := map[string]bool{}
	for _, entry := range Catalog {
		if entry.Kind != "region" {
			continue
		}
		if view, _, ok := splitBlockEntry(entry.Entry); ok {
			views[view] = true
		}
	}
	return slices.Sorted(maps.Keys(views))
}

// xsdAttr writes one block attribute from its catalog type: int becomes an
// integer restriction with min/max, enum becomes an enumeration.
func xsdAttr(b *strings.Builder, attr types.ThemeCatalogAttr, indent string) {
	switch attr.Type {
	case "int":
		fmt.Fprintf(b, "%s<xs:attribute name=\"%s\">\n", indent, attr.Name)
		fmt.Fprintf(b, "%s  <xs:simpleType>\n", indent)
		fmt.Fprintf(b, "%s    <xs:restriction base=\"xs:integer\">\n", indent)
		fmt.Fprintf(b, "%s      <xs:minInclusive value=\"%d\" />\n", indent, attr.Min)
		if attr.Max != 0 {
			fmt.Fprintf(b, "%s      <xs:maxInclusive value=\"%d\" />\n", indent, attr.Max)
		}
		fmt.Fprintf(b, "%s    </xs:restriction>\n", indent)
		fmt.Fprintf(b, "%s  </xs:simpleType>\n", indent)
		fmt.Fprintf(b, "%s</xs:attribute>\n", indent)
	case "enum":
		fmt.Fprintf(b, "%s<xs:attribute name=\"%s\">\n", indent, attr.Name)
		fmt.Fprintf(b, "%s  <xs:simpleType>\n", indent)
		fmt.Fprintf(b, "%s    <xs:restriction base=\"xs:string\">\n", indent)
		for _, v := range attr.Values {
			fmt.Fprintf(b, "%s      <xs:enumeration value=\"%s\" />\n", indent, v)
		}
		fmt.Fprintf(b, "%s    </xs:restriction>\n", indent)
		fmt.Fprintf(b, "%s  </xs:simpleType>\n", indent)
		fmt.Fprintf(b, "%s</xs:attribute>\n", indent)
	default:
		fmt.Fprintf(b, "%s<xs:attribute name=\"%s\" type=\"xs:string\" />\n", indent, attr.Name)
	}
}

// xsdEnum writes a string restriction enumerating one name set, sorted.
func xsdEnum(b *strings.Builder, names map[string]bool, indent int) {
	pad := strings.Repeat(" ", indent)
	b.WriteString(pad + "<xs:simpleType>\n")
	b.WriteString(pad + "  <xs:restriction base=\"xs:string\">\n")
	for _, name := range slices.Sorted(maps.Keys(names)) {
		fmt.Fprintf(b, "%s    <xs:enumeration value=\"%s\" />\n", pad, name)
	}
	b.WriteString(pad + "  </xs:restriction>\n")
	b.WriteString(pad + "</xs:simpleType>\n")
}
