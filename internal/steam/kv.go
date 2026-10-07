package steam

import (
	"fmt"
	"strings"
)

// KV is a node of Valve's text KeyValues format (.vdf/.acf). A node is either
// a string value or a map of child nodes.
type KV struct {
	Value    string
	Children map[string]*KV
}

// Get walks a path of keys, case-insensitively like Steam does. Returns nil if
// any segment is missing.
func (n *KV) Get(path ...string) *KV {
	cur := n
	for _, key := range path {
		if cur == nil || cur.Children == nil {
			return nil
		}
		next, ok := cur.Children[key]
		if !ok {
			next = nil
			for k, v := range cur.Children {
				if strings.EqualFold(k, key) {
					next = v
					break
				}
			}
		}
		cur = next
	}
	return cur
}

// String returns the value at path, or "" if absent.
func (n *KV) String(path ...string) string {
	if v := n.Get(path...); v != nil {
		return v.Value
	}
	return ""
}

// ParseKV parses text KeyValues into a root map node.
func ParseKV(data string) (*KV, error) {
	p := &kvParser{src: data}
	root := &KV{Children: map[string]*KV{}}
	if err := p.parseInto(root, true); err != nil {
		return nil, err
	}
	return root, nil
}

type kvParser struct {
	src string
	pos int
}

func (p *kvParser) skip() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == 0:
			p.pos++ // NUL included: live PICS buffers are NUL-terminated
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		default:
			return
		}
	}
}

// token returns the next token: "{", "}", or a string. ok=false at EOF.
func (p *kvParser) token() (tok string, isBrace bool, ok bool) {
	p.skip()
	if p.pos >= len(p.src) {
		return "", false, false
	}
	c := p.src[p.pos]
	if c == '{' || c == '}' {
		p.pos++
		return string(c), true, true
	}
	if c == '"' {
		p.pos++
		var b strings.Builder
		for p.pos < len(p.src) && p.src[p.pos] != '"' {
			if p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
				p.pos++
				switch p.src[p.pos] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(p.src[p.pos])
				}
			} else {
				b.WriteByte(p.src[p.pos])
			}
			p.pos++
		}
		p.pos++ // closing quote
		return b.String(), false, true
	}
	start := p.pos
	for p.pos < len(p.src) && !strings.ContainsRune(" \t\r\n{}\"", rune(p.src[p.pos])) {
		p.pos++
	}
	return p.src[start:p.pos], false, true
}

func (p *kvParser) parseInto(node *KV, top bool) error {
	for {
		key, brace, ok := p.token()
		if !ok {
			if top {
				return nil
			}
			return fmt.Errorf("unexpected end of input")
		}
		if brace {
			if key == "}" && !top {
				return nil
			}
			return fmt.Errorf("unexpected %q at %d", key, p.pos)
		}
		val, vbrace, ok := p.token()
		if !ok {
			return fmt.Errorf("missing value for %q", key)
		}
		if vbrace && val == "{" {
			child := &KV{Children: map[string]*KV{}}
			if err := p.parseInto(child, false); err != nil {
				return err
			}
			node.Children[key] = child
		} else if vbrace {
			return fmt.Errorf("unexpected %q after %q", val, key)
		} else {
			node.Children[key] = &KV{Value: val}
		}
	}
}
