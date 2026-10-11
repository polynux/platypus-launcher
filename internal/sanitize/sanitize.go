package sanitize

import (
	"html"
	"net/url"
	"strings"
)

// Allowlist for store HTML (descriptions, requirements). The UI renders the
// result as markup, so anything not on the list is escaped or dropped:
// p, br, ul, ol, li, b, strong, i, em, h2-h4 and a[href https]. No
// img/iframe/style/script; links open through openExternal in the UI.
var allowed = map[string]bool{
	"p": true, "br": true, "ul": true, "ol": true, "li": true,
	"b": true, "strong": true, "i": true, "em": true,
	"h2": true, "h3": true, "h4": true, "a": true,
}

// dropContent are elements whose content goes with the tag (script/style can
// otherwise leak code as text; media/frame elements carry nothing to keep).
var dropContent = map[string]bool{
	"script": true, "style": true, "iframe": true, "img": true,
	"object": true, "embed": true, "link": true, "meta": true,
}

const maxInputRunes = 200_000

// HTML sanitizes store HTML to the allowlist above. Text is escaped,
// disallowed tags are dropped (script/style and media elements with their
// content), and links keep only an https href. Unclosed tags are closed;
// unparseable input degrades to escaped text. The result is safe for the UI
// to render as markup.
func HTML(input string) string {
	runes := []rune(input)
	if len(runes) > maxInputRunes {
		runes = runes[:maxInputRunes]
	}
	s := string(runes)
	var b strings.Builder
	b.Grow(len(s))
	stack := []string{}
	i := 0
	for i < len(s) {
		if s[i] != '<' {
			next := strings.IndexByte(s[i:], '<')
			var text string
			if next < 0 {
				text = s[i:]
				i = len(s)
			} else {
				text = s[i : i+next]
				i += next
			}
			b.WriteString(html.EscapeString(text))
			continue
		}
		// Comment: drop it.
		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			b.WriteString(html.EscapeString(s[i:]))
			break
		}
		raw := s[i+1 : i+end]
		i += end + 1
		tag, closing, ok := splitTag(raw)
		if !ok {
			continue
		}
		name := strings.ToLower(tag)
		if dropContent[name] {
			if !closing && name != "img" {
				i = skipUntilClose(s, i, name)
			}
			continue
		}
		if !allowed[name] {
			continue
		}
		if name == "br" {
			b.WriteString("<br>")
			continue
		}
		if closing {
			// Close the matching open element, dropping stray closers.
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k] == name {
					for len(stack) > k {
						top := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						b.WriteString("</" + top + ">")
					}
					break
				}
			}
			continue
		}
		if name == "a" {
			href := linkHref(raw)
			if href == "" {
				stack = append(stack, "a")
				b.WriteString("<a>")
			} else {
				stack = append(stack, "a")
				b.WriteString(`<a href="` + html.EscapeString(href) + `">`)
			}
			continue
		}
		stack = append(stack, name)
		b.WriteString("<" + name + ">")
	}
	for k := len(stack) - 1; k >= 0; k-- {
		b.WriteString("</" + stack[k] + ">")
	}
	return b.String()
}

func splitTag(raw string) (name string, closing bool, ok bool) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", false, false
	}
	if strings.HasPrefix(t, "/") {
		t = strings.TrimSpace(t[1:])
		closing = true
	}
	if strings.HasSuffix(t, "/") {
		t = strings.TrimSpace(t[:len(t)-1])
	}
	if i := strings.IndexAny(t, " \t\n\r\f/"); i >= 0 {
		name = t[:i]
	} else {
		name = t
	}
	if name == "" {
		return "", false, false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", false, false
		}
	}
	return name, closing, true
}

// linkHref keeps an <a> href only when it is an https URL.
func linkHref(raw string) string {
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "href")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(raw[idx+4:])
	if !strings.HasPrefix(rest, "=") {
		return ""
	}
	rest = strings.TrimSpace(rest[1:])
	var val string
	if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
		q := rest[0]
		end := strings.IndexByte(rest[1:], q)
		if end < 0 {
			return ""
		}
		val = rest[1 : 1+end]
	} else {
		end := strings.IndexAny(rest, " \t\n\r\f>")
		if end < 0 {
			val = rest
		} else {
			val = rest[:end]
		}
	}
	val = strings.TrimSpace(val)
	u, err := url.Parse(val)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}

func skipUntilClose(s string, i int, name string) int {
	lower := strings.ToLower(s)
	closeTag := "</" + name
	for i < len(s) {
		lt := strings.Index(lower[i:], "<")
		if lt < 0 {
			return len(s)
		}
		j := i + lt
		if strings.HasPrefix(lower[j:], closeTag) {
			end := strings.IndexByte(s[j:], '>')
			if end < 0 {
				return len(s)
			}
			return j + end + 1
		}
		i = j + 1
	}
	return i
}

// HTTPSURL keeps an https URL and drops anything else.
func HTTPSURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}

// MediaURL keeps an https URL or a loopback http URL (the local art server:
// 127.0.0.1 or localhost). Everything else is dropped.
func MediaURL(raw string) string {
	s := strings.TrimSpace(raw)
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme == "https" {
		return u.String()
	}
	if u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") {
		return u.String()
	}
	return ""
}

// Clip cuts s to max runes after trimming.
func Clip(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > max {
		return string(r[:max])
	}
	return s
}

// Strings clips each entry, drops empties, dedupes and bounds the list.
func Strings(in []string, maxItems, maxRunes int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		if len(out) >= maxItems {
			break
		}
		s = Clip(s, maxRunes)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
