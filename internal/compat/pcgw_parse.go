package compat

import (
	"regexp"
	"strings"
	"time"
)

// The PCGamingWiki page is read from its wikitext: the Infobox (Steam AppID),
// the Availability section (per-store DRM rows and DRM notes), and the
// Middleware template (anti-cheat). The parser is tolerant: a template it
// can't read is skipped, a value it can't name is dropped, and a fact that
// isn't there stays unknown. It never guesses.

// WikiFacts is what the parser could read from one page.
type WikiFacts struct {
	// SteamAppID is the page's main Steam AppID; SteamSide the other AppIDs
	// the page covers (regional or edition versions).
	SteamAppID string
	SteamSide  []string
	// DRM are the DRM names listed for the Windows version, in page order.
	DRM []string
	// Removed is DRM the page says was removed from the Windows version, with
	// the date. A removal without a date is left out.
	Removed []RemovedDRM
	// Online is "always" when a DRM note says the game is always online,
	// "launch" when it needs to be online to start, "" otherwise.
	Online string
	// AntiCheat are the anti-cheat systems named in the Middleware template.
	AntiCheat []string
}

// RemovedDRM is a DRM that was taken out of a version, on a day (YYYY-MM-DD).
type RemovedDRM struct {
	Name string
	Date string
}

// tmpl is one template call found in wikitext.
type tmpl struct {
	pos   []string          // positional arguments, trimmed
	named map[string]string // named arguments, keys lower-cased and trimmed
	end   int               // offset just past the closing braces
}

// findTemplates returns every call of the template name (case and underscore
// insensitive), in text order, nested calls included.
func findTemplates(text, name string) []tmpl {
	want := normTemplateName(name)
	var out []tmpl
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], "{{")
		if j < 0 {
			break
		}
		start := i + j
		end := matchBraces(text, start)
		if end < 0 {
			break
		}
		parts := splitTop(text[start+2 : end-2])
		if len(parts) > 0 && normTemplateName(parts[0]) == want {
			out = append(out, toTmpl(parts[1:], end))
		}
		i = start + 2 // templates nest: a wrapper's arguments are searched too
	}
	return out
}

// matchBraces returns the offset just past the "}}" closing the "{{" at start,
// or -1 when it never closes.
func matchBraces(text string, start int) int {
	depth := 0
	for i := start; i < len(text)-1; i++ {
		switch {
		case text[i] == '{' && text[i+1] == '{':
			depth++
			i++
		case text[i] == '}' && text[i+1] == '}':
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// splitTop splits a template body on its "|" separators that are not inside a
// nested template or link.
func splitTop(body string) []string {
	var parts []string
	depth, last := 0, 0
	for i := 0; i < len(body); i++ {
		if i+1 < len(body) && (body[i:i+2] == "{{" || body[i:i+2] == "[[") {
			depth++
			i++
			continue
		}
		if i+1 < len(body) && (body[i:i+2] == "}}" || body[i:i+2] == "]]") {
			depth--
			i++
			continue
		}
		if body[i] == '|' && depth == 0 {
			parts = append(parts, body[last:i])
			last = i + 1
		}
	}
	return append(parts, body[last:])
}

// toTmpl sorts the arguments of a call into positional and named ones. A part
// is named when its "=" comes before any template or link.
func toTmpl(args []string, end int) tmpl {
	t := tmpl{named: map[string]string{}, end: end}
	for _, a := range args {
		eq := strings.Index(a, "=")
		markup := strings.Index(a, "{{")
		if link := strings.Index(a, "[["); link >= 0 && (markup < 0 || link < markup) {
			markup = link
		}
		if eq >= 0 && (markup < 0 || eq < markup) {
			t.named[strings.ToLower(strings.TrimSpace(a[:eq]))] = strings.TrimSpace(a[eq+1:])
			continue
		}
		t.pos = append(t.pos, strings.TrimSpace(a))
	}
	return t
}

func normTemplateName(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "_", " ")
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// sectionText is the wikitext of the level-2 section named heading, up to the
// next level-2 heading. "" when there is no such section.
func sectionText(text, heading string) string {
	loc := regexp.MustCompile(`(?m)^==\s*` + regexp.QuoteMeta(heading) + `\s*==\s*$`).FindStringIndex(text)
	if loc == nil {
		return ""
	}
	rest := text[loc[1]:]
	if next := regexp.MustCompile(`(?m)^==[^=]`).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	return rest
}

var (
	linkRe    = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
	drmCell   = regexp.MustCompile(`^\{\{DRM\|([^{}|]*)(?:\|[^{}]*)?\}\}$`)
	refRe     = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>|<ref[^>]*/>|<[^>]+>`)
	removedRe = regexp.MustCompile(`removed from the ([A-Za-z0-9 ]+?) version on ([A-Z][a-z]+ \d{1,2}, \d{4})`)
)

// plain turns a wikitext value into its name: links are unwrapped and
// references dropped. A value with any template left in it is "" (unknown):
// a template's meaning is not guessed from its text.
func plain(s string) string {
	s = linkRe.ReplaceAllString(s, "$1")
	s = refRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if strings.ContainsAny(s, "{}[]<>") || len(s) > maxName {
		return ""
	}
	return s
}

// drmName reads the DRM named in an Availability row or a DRM note: the
// argument of {{DRM|...}} or a plain name.
func drmName(s string) string {
	if m := drmCell.FindStringSubmatch(strings.TrimSpace(s)); m != nil {
		s = m[1]
	}
	return plain(s)
}

// maxName caps a DRM or anti-cheat name read from a page.
const maxName = 60

// isWindowsRow says whether an Availability row is for the Windows version
// (its operating system is the last positional argument).
func isWindowsRow(t tmpl) bool {
	if len(t.pos) == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(t.pos[len(t.pos)-1]), "windows")
}

// ParseWikitext reads the facts of a page's wikitext. An empty result means the
// page had nothing readable.
func ParseWikitext(text string) WikiFacts {
	var f WikiFacts
	if info := findTemplates(text, "Infobox game"); len(info) > 0 {
		f.SteamAppID = info[0].named["steam appid"]
		for _, id := range strings.Split(info[0].named["steam appid side"], ",") {
			if id = strings.TrimSpace(id); validAppID(id) {
				f.SteamSide = append(f.SteamSide, id)
			}
		}
	}

	avail := sectionText(text, "Availability")
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			f.DRM = append(f.DRM, name)
		}
	}
	// Availability rows: store, store ID, DRM, notes, version, OS.
	for _, row := range findTemplates(avail, "Availability/row") {
		if isWindowsRow(row) && len(row.pos) > 2 {
			add(drmName(row.pos[2]))
		}
	}
	// DRM notes in the prose: {{DRM|Steam}}, with offline=false for always online.
	for _, d := range findTemplates(avail, "DRM") {
		if len(d.pos) == 0 {
			continue
		}
		add(drmName(d.pos[0]))
		switch d.named["offline"] {
		case "false":
			f.Online = "always"
		case "postlaunch":
			if f.Online == "" {
				f.Online = "launch"
			}
		}
	}
	// Removed DRM: the sentence after the template names the version and day.
	for _, r := range findTemplates(avail, "Removed DRM") {
		name := ""
		if len(r.pos) > 0 {
			name = plain(r.pos[0])
		}
		line := avail[r.end:]
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line = line[:nl]
		}
		m := removedRe.FindStringSubmatch(line)
		if name == "" || m == nil || !strings.EqualFold(strings.TrimSpace(m[1]), "Windows") {
			continue
		}
		day, err := time.Parse("January 2, 2006", m[2])
		if err != nil {
			continue
		}
		f.Removed = append(f.Removed, RemovedDRM{Name: name, Date: day.Format("2006-01-02")})
	}

	if mw := findTemplates(text, "Middleware"); len(mw) > 0 {
		seenAC := map[string]bool{}
		for _, n := range strings.Split(mw[0].named["anticheat"], ",") {
			if name := plain(n); name != "" && !seenAC[name] {
				seenAC[name] = true
				f.AntiCheat = append(f.AntiCheat, name)
			}
		}
	}
	return f
}
