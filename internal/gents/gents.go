// Package gents generates the TypeScript bridge types for the Electron shell
// from the Go core: interfaces for the structs the App's exported methods
// exchange, and the typed method list the bridge invokes by name.
//
// The method filter mirrors internal/corerpc's servable check, so the
// generated CoreApi is exactly the RPC surface initialize advertises.
// Parameter names come from the Go source (reflection cannot see them); give
// Generate the target's source directory.
//
// The output (frontend/src/gen/) is checked in; TestGeneratedUpToDate fails
// when it is stale. Regenerate with `go run ./cmd/gen-ts` from the
// repository root.
package gents

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Packages whose structs are part of the bridge. A method signature naming a
// struct from anywhere else is an error: the generator must learn it first.
const (
	pkgTypes  = "platypus/internal/types"
	pkgChat   = "platypus/internal/chat"
	pkgSocial = "platypus/internal/social"
)

var namespaces = map[string]string{
	pkgTypes:  "types",
	pkgChat:   "chat",
	pkgSocial: "social",
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// rawMessageType is encoding/json.RawMessage: bytes on the wire but opaque
// JSON here. Compared by type identity because recent toolchains alias it to
// another bytes type.
var rawMessageType = reflect.TypeOf(json.RawMessage{})

// Param is one positional argument of a served method.
type Param struct {
	Name string // the Go parameter name
	Type string // the TypeScript type
}

// Method is one served method: its name, parameters and resolved result.
type Method struct {
	Name   string
	Params []Param
	Result string // TS type of the resolved value; "void" when none
}

// Methods reflects over target (typically *app.App) and returns the served
// methods sorted by name: the same set and order internal/corerpc advertises
// from initialize. Parameter names are arg0, arg1, ...; use Generate for the
// real names.
func Methods(target any) ([]Method, error) {
	methods, _, err := generate(target, "")
	return methods, err
}

// Generate reflects over target and returns the two checked-in files:
// models.ts (namespaced interfaces) and api.ts (the typed method list).
// srcDir is the target's source directory, used for parameter names; when it
// is empty the parameters are arg0, arg1, ....
func Generate(target any, srcDir string) (models, api string, err error) {
	methods, g, err := generate(target, srcDir)
	if err != nil {
		return "", "", err
	}
	return g.modelsFile(), g.apiFile(methods), nil
}

func generate(target any, srcDir string) ([]Method, *gen, error) {
	t := reflect.TypeOf(target)
	if t == nil {
		return nil, nil, fmt.Errorf("gents: nil target")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("gents: target must be a struct, got %s", t.Kind())
	}
	names, err := paramNames(srcDir, t.Name())
	if err != nil {
		return nil, nil, err
	}
	g := &gen{structs: map[string]reflect.Type{}}
	pt := reflect.PointerTo(t)
	var methods []Method
	for i := 0; i < pt.NumMethod(); i++ {
		m := pt.Method(i)
		if !servable(m.Type) {
			continue
		}
		params, err := g.params(m, names[m.Name])
		if err != nil {
			return nil, nil, fmt.Errorf("gents: method %s: %w", m.Name, err)
		}
		result, err := g.result(m)
		if err != nil {
			return nil, nil, fmt.Errorf("gents: method %s: %w", m.Name, err)
		}
		methods = append(methods, Method{Name: m.Name, Params: params, Result: result})
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })
	// Walk the fields of every collected type so nested types are emitted
	// too. New types append to g.order and are visited in turn; re-resolving
	// a recorded type is a no-op, so this terminates.
	for i := 0; i < len(g.order); i++ {
		t := g.structs[g.order[i]]
		if t.Kind() == reflect.Struct {
			for _, f := range flatFields(t) {
				if _, err := g.tsType(f.Type, true); err != nil {
					return nil, nil, fmt.Errorf("gents: type %s field %s: %w", t.Name(), f.Name, err)
				}
			}
			continue
		}
		if _, err := g.tsType(t, true); err != nil {
			return nil, nil, fmt.Errorf("gents: type %s: %w", t.Name(), err)
		}
	}
	return methods, g, nil
}

// servable mirrors internal/corerpc's check: the method's parameters and
// results must travel as JSON, and it returns at most a value plus an error.
func servable(t reflect.Type) bool {
	ok := func(t reflect.Type) bool {
		switch t.Kind() {
		case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
			return false
		}
		return true
	}
	// In(0) is the receiver.
	for i := 1; i < t.NumIn(); i++ {
		if !ok(t.In(i)) {
			return false
		}
	}
	switch t.NumOut() {
	case 0:
	case 1:
		if !ok(t.Out(0)) && t.Out(0) != errorType {
			return false
		}
	case 2:
		if t.Out(1) != errorType || !ok(t.Out(0)) {
			return false
		}
	default:
		return false
	}
	return true
}

// paramNames reads the receiver methods' parameter names from the Go source
// in dir (which must hold the struct's package). It returns method name to
// parameter names.
func paramNames(dir, recv string) (map[string][]string, error) {
	out := map[string][]string{}
	if dir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gents: reading %s: %w", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("gents: parsing %s: %w", e.Name(), err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name == nil {
				continue
			}
			if !isRecv(fn.Recv, recv) {
				continue
			}
			var params []string
			for _, field := range fn.Type.Params.List {
				for _, n := range field.Names {
					params = append(params, n.Name)
				}
				if len(field.Names) == 0 {
					params = append(params, "")
				}
			}
			out[fn.Name.Name] = params
		}
	}
	return out, nil
}

// isRecv reports whether the receiver list is (x Recv) for some x.
func isRecv(recv *ast.FieldList, name string) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}
	switch e := recv.List[0].Type.(type) {
	case *ast.Ident:
		return e.Name == name
	case *ast.StarExpr:
		id, ok := e.X.(*ast.Ident)
		return ok && id.Name == name
	}
	return false
}

type gen struct {
	// structs collects every bridge struct and alias by "pkgPath.Name", in
	// first-seen order (emission sorts them).
	structs map[string]reflect.Type
	order   []string
}

func (g *gen) params(m reflect.Method, names []string) ([]Param, error) {
	t := m.Type
	var out []Param
	for i := 1; i < t.NumIn(); i++ {
		name := fmt.Sprintf("arg%d", i-1)
		if i-1 < len(names) && names[i-1] != "" && names[i-1] != "_" {
			name = names[i-1]
		}
		ts, err := g.tsType(t.In(i), true)
		if err != nil {
			return nil, err
		}
		out = append(out, Param{Name: name, Type: ts})
	}
	return out, nil
}

func (g *gen) result(m reflect.Method) (string, error) {
	t := m.Type
	switch t.NumOut() {
	case 0:
		return "void", nil
	case 1:
		if t.Out(0) == errorType {
			return "void", nil
		}
		return g.tsType(t.Out(0), true)
	case 2:
		return g.tsType(t.Out(0), true)
	default:
		return "", fmt.Errorf("too many results")
	}
}

// tsType maps a Go type to TypeScript. With collect it records named bridge
// structs and aliases along the way; without, it only resolves them (for
// emission, after collection).
func (g *gen) tsType(t reflect.Type, collect bool) (string, error) {
	if t == errorType {
		return "unknown", nil
	}
	// encoding/json.RawMessage is bytes on the wire but opaque JSON here.
	if t == rawMessageType {
		return "unknown", nil
	}
	// time.Time travels as RFC 3339 text; time.Duration as a count.
	if t.PkgPath() == "time" {
		switch t.Name() {
		case "Time":
			return "string", nil
		case "Duration":
			return "number", nil
		}
	}
	switch t.Kind() {
	case reflect.Bool:
		return g.primitive(t, "boolean", collect)
	case reflect.String:
		return g.primitive(t, "string", collect)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		// A lone byte is a number like the rest; a byte slice is text
		// (handled under Slice).
		return g.primitive(t, "number", collect)
	case reflect.Pointer:
		elem, err := g.tsType(t.Elem(), collect)
		if err != nil {
			return "", err
		}
		return elem + " | null", nil
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return "string", nil
		}
		elem, err := g.tsType(t.Elem(), collect)
		if err != nil {
			return "", err
		}
		if strings.Contains(elem, "|") {
			return "Array<" + elem + ">", nil
		}
		return elem + "[]", nil
	case reflect.Map:
		key, err := g.mapKey(t.Key())
		if err != nil {
			return "", err
		}
		elem, err := g.tsType(t.Elem(), collect)
		if err != nil {
			return "", err
		}
		rec := "Record<" + key + ", " + elem + ">"
		if t.Name() != "" {
			return g.resolve(t, rec, collect)
		}
		return rec, nil
	case reflect.Interface:
		// any / interface{}: the value is JSON, its shape unknown.
		return "unknown", nil
	case reflect.Struct:
		if t.Name() == "" {
			return "", fmt.Errorf("anonymous struct %s needs a named Go type", t)
		}
		return g.resolve(t, "", collect)
	default:
		return "", fmt.Errorf("cannot map %s (kind %s) to TypeScript", t, t.Kind())
	}
}

// primitive maps a bool/number/string kind, resolving named types (e.g.
// StoreType) to their alias reference.
func (g *gen) primitive(t reflect.Type, ts string, collect bool) (string, error) {
	if t.Name() != "" && t.PkgPath() != "" {
		return g.resolve(t, ts, collect)
	}
	return ts, nil
}

func (g *gen) mapKey(t reflect.Type) (string, error) {
	switch t.Kind() {
	case reflect.String:
		return "string", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number", nil
	default:
		return "", fmt.Errorf("cannot map %s (kind %s) to a Record key", t, t.Kind())
	}
}

// resolve returns the namespaced reference of a named bridge type, recording
// it first when collect is set. Structs (underlying "") are emitted as
// interfaces; named primitives and maps as aliases for underlying.
func (g *gen) resolve(t reflect.Type, underlying string, collect bool) (string, error) {
	ns, ok := namespaces[t.PkgPath()]
	if !ok {
		return "", fmt.Errorf("type %s.%s is outside the bridge packages (types, chat, social)", t.PkgPath(), t.Name())
	}
	if collect {
		key := t.PkgPath() + "." + t.Name()
		if prev, seen := g.structs[key]; seen {
			if prev != t {
				return "", fmt.Errorf("type name %s.%s resolves twice", t.PkgPath(), t.Name())
			}
		} else {
			g.structs[key] = t
			g.order = append(g.order, key)
		}
	}
	_ = underlying
	return ns + "." + t.Name(), nil
}

const header = `// Code generated by ` + "`go run ./cmd/gen-ts`" + ` from the Go core. DO NOT EDIT.
// The App's exported methods are the RPC surface (internal/corerpc); these
// types are what they exchange. Nil slices and maps arrive as null: the
// frontend normalizes them at the edge (see effect/runtime.ts).
`

// modelsFile emits the namespaced interfaces and aliases, sorted for stability.
func (g *gen) modelsFile() string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	// Group collected types by namespace.
	byNS := map[string][]reflect.Type{}
	for _, key := range g.order {
		t := g.structs[key]
		ns := namespaces[t.PkgPath()]
		byNS[ns] = append(byNS[ns], t)
	}
	var nss []string
	for ns := range byNS {
		nss = append(nss, ns)
	}
	sort.Strings(nss)
	for i, ns := range nss {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("export namespace " + ns + " {\n")
		types := byNS[ns]
		sort.Slice(types, func(i, j int) bool { return types[i].Name() < types[j].Name() })
		for _, t := range types {
			g.emitType(&b, t)
		}
		b.WriteString("}\n")
	}
	return b.String()
}

// emitType writes one interface or alias inside its namespace.
func (g *gen) emitType(b *strings.Builder, t reflect.Type) {
	if t.Kind() == reflect.Struct {
		b.WriteString("\n  export interface " + t.Name() + " {\n")
		for _, f := range flatFields(t) {
			tag := f.Tag.Get("json")
			name, optional := fieldName(f, tag)
			ts, err := g.tsType(f.Type, false)
			if err != nil {
				// Collection already succeeded; only an anonymous struct
				// field can fail, and flatFields inlined those. Unreachable.
				panic(err)
			}
			mark := ""
			if optional {
				mark = "?"
			}
			b.WriteString("    " + name + mark + ": " + ts + ";\n")
		}
		b.WriteString("  }\n")
		return
	}
	b.WriteString("\n  export type " + t.Name() + " = " + g.aliasOf(t) + ";\n")
}

// aliasOf resolves the TypeScript of a named primitive or map without
// collecting (its dependencies are collected already).
func (g *gen) aliasOf(t reflect.Type) string {
	if t.Kind() == reflect.Map {
		key, err := g.mapKey(t.Key())
		if err != nil {
			panic(err)
		}
		elem, err := g.tsType(t.Elem(), false)
		if err != nil {
			panic(err)
		}
		return "Record<" + key + ", " + elem + ">"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	default:
		return "number"
	}
}

// flatFields returns a struct's fields in declaration order, inlining embedded
// structs (an embedded field without its own JSON name), skipping unexported
// fields and `json:"-"`.
func flatFields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue // unexported
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct && jsonName(tag) == "" {
			out = append(out, flatFields(f.Type)...)
			continue
		}
		if f.PkgPath != "" {
			continue // unexported embedded value: no JSON of its own
		}
		out = append(out, f)
	}
	return out
}

// fieldName splits a json tag into the wire name and whether omitempty makes
// it optional.
func fieldName(f reflect.StructField, tag string) (string, bool) {
	name, optional := "", false
	if tag != "" {
		parts := strings.Split(tag, ",")
		name = parts[0]
		for _, o := range parts[1:] {
			if o == "omitempty" {
				optional = true
			}
		}
	}
	if name == "" {
		name = f.Name
	}
	return name, optional
}

func jsonName(tag string) string {
	if tag == "" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

// apiFile emits the typed method list the bridge invokes by name.
func (g *gen) apiFile(methods []Method) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\nimport type { chat, social, types } from './models';\n")
	b.WriteString("\n/** One exported method of the Go core, invoked by name over the bridge. */\n")
	b.WriteString("export interface CoreApi {\n")
	for _, m := range methods {
		var params []string
		for _, p := range m.Params {
			params = append(params, p.Name+": "+p.Type)
		}
		b.WriteString("  " + m.Name + "(" + strings.Join(params, ", ") + "): Promise<" + m.Result + ">;\n")
	}
	b.WriteString("}\n")
	b.WriteString("\n/** Every method the core serves, for validating bridge calls. */\n")
	b.WriteString("export const coreMethods: (keyof CoreApi)[] = [\n")
	for _, m := range methods {
		b.WriteString("  '" + m.Name + "',\n")
	}
	b.WriteString("];\n")
	return b.String()
}
