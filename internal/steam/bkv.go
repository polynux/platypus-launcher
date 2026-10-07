// Binary KeyValues parser. PICS product-info buffers arrive as binary KV,
// not text (the text parser in kv.go rejects them at the first NUL), so the
// CM layer decodes them here into the same KV shape the depot selector
// already walks. Layout, matching SteamKit's binary reader: each node is a
// type byte, a NUL-terminated key, then a payload by type (None nests
// children until an End marker); numbers are little-endian.
package steam

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// Node types of binary KeyValues.
const (
	bkvNone    = 0
	bkvString  = 1
	bkvInt32   = 2
	bkvFloat32 = 3
	bkvPointer = 4
	bkvWString = 5
	bkvColor   = 6
	bkvUInt64  = 7
	bkvEnd     = 8
)

// ParseBinaryKV parses binary KeyValues into a root map node.
func ParseBinaryKV(data []byte) (*KV, error) {
	p := &bkvParser{src: data}
	root := &KV{Children: map[string]*KV{}}
	if err := p.nodes(root); err != nil {
		return nil, err
	}
	return root, nil
}

type bkvParser struct {
	src []byte
	pos int
}

func (p *bkvParser) byte() (byte, error) {
	if p.pos >= len(p.src) {
		return 0, fmt.Errorf("unexpected end of input")
	}
	b := p.src[p.pos]
	p.pos++
	return b, nil
}

func (p *bkvParser) cstring() (string, error) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != 0 {
		p.pos++
	}
	if p.pos >= len(p.src) {
		return "", fmt.Errorf("unterminated string")
	}
	s := string(p.src[start:p.pos])
	p.pos++ // NUL
	return s, nil
}

func (p *bkvParser) fixed(n int) ([]byte, error) {
	if p.pos+n > len(p.src) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	b := p.src[p.pos : p.pos+n]
	p.pos += n
	return b, nil
}

// nodes reads sibling nodes into parent until an End marker or EOF.
func (p *bkvParser) nodes(parent *KV) error {
	for p.pos < len(p.src) {
		typ, err := p.byte()
		if err != nil {
			return err
		}
		if typ == bkvEnd {
			return nil
		}
		key, err := p.cstring()
		if err != nil {
			return err
		}
		node, err := p.value(typ)
		if err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
		parent.Children[key] = node
	}
	return nil
}

func (p *bkvParser) value(typ byte) (*KV, error) {
	switch typ {
	case bkvNone:
		n := &KV{Children: map[string]*KV{}}
		if err := p.nodes(n); err != nil {
			return nil, err
		}
		return n, nil
	case bkvString:
		s, err := p.cstring()
		if err != nil {
			return nil, err
		}
		return &KV{Value: s}, nil
	case bkvInt32:
		b, err := p.fixed(4)
		if err != nil {
			return nil, err
		}
		return &KV{Value: strconv.FormatInt(int64(int32(binary.LittleEndian.Uint32(b))), 10)}, nil
	case bkvFloat32:
		b, err := p.fixed(4)
		if err != nil {
			return nil, err
		}
		return &KV{Value: strconv.FormatFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), 'g', -1, 32)}, nil
	case bkvPointer:
		b, err := p.fixed(4)
		if err != nil {
			return nil, err
		}
		return &KV{Value: strconv.FormatUint(uint64(binary.LittleEndian.Uint32(b)), 10)}, nil
	case bkvWString:
		start := p.pos
		for {
			if p.pos+2 > len(p.src) {
				return nil, fmt.Errorf("unterminated wide string")
			}
			lo := binary.LittleEndian.Uint16(p.src[p.pos:])
			p.pos += 2
			if lo == 0 {
				break
			}
		}
		raw := p.src[start : p.pos-2]
		runes := make([]rune, 0, len(raw)/2)
		for i := 0; i+2 <= len(raw); i += 2 {
			runes = append(runes, rune(binary.LittleEndian.Uint16(raw[i:])))
		}
		return &KV{Value: string(runes)}, nil
	case bkvColor:
		b, err := p.fixed(4)
		if err != nil {
			return nil, err
		}
		return &KV{Value: fmt.Sprintf("%d %d %d %d", b[0], b[1], b[2], b[3])}, nil
	case bkvUInt64:
		b, err := p.fixed(8)
		if err != nil {
			return nil, err
		}
		return &KV{Value: strconv.FormatUint(binary.LittleEndian.Uint64(b), 10)}, nil
	default:
		return nil, fmt.Errorf("unknown node type %d", typ)
	}
}
