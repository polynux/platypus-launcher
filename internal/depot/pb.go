// Minimal protobuf wire support for the few messages the downloader
// needs (content_manifest.proto and the CM/client messages in
// internal/steamcm). Only varints, fixed64/32 and length-delimited
// fields; Steam's messages stay in that subset. Field numbers come from
// SteamDatabase/Protobufs, never from memory (see the .proto notes on
// each caller).
package depot

import (
	"fmt"
)

// Field holds one decoded field's values: ints for varint/fixed,
// raw bytes for length-delimited.
type Field struct {
	Ints  []uint64
	Bytes [][]byte
}

// Msg maps field number -> values.
type Msg map[uint32]*Field

func (m Msg) ints(f uint32) []uint64 {
	if e := m[f]; e != nil {
		return e.Ints
	}
	return nil
}

// Int returns the first varint/fixed value of a field, or 0.
func (m Msg) Int(f uint32) uint64 {
	v := m.ints(f)
	if len(v) == 0 {
		return 0
	}
	return v[0]
}

// Bytes returns the first length-delimited value of a field, or nil.
func (m Msg) Bytes(f uint32) []byte {
	if e := m[f]; e != nil && len(e.Bytes) > 0 {
		return e.Bytes[0]
	}
	return nil
}

// AllBytes returns every length-delimited value of a field.
func (m Msg) AllBytes(f uint32) [][]byte {
	if e := m[f]; e != nil {
		return e.Bytes
	}
	return nil
}

// Decode parses a protobuf message into fields.
func Decode(data []byte) (Msg, error) {
	m := Msg{}
	i := 0
	for i < len(data) {
		key, n := varint(data[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad protobuf key at %d", i)
		}
		i += n
		field := uint32(key >> 3)
		wt := key & 7
		e := m[field]
		if e == nil {
			e = &Field{}
			m[field] = e
		}
		switch wt {
		case 0: // varint
			v, n := varint(data[i:])
			if n <= 0 {
				return nil, fmt.Errorf("bad varint at %d", i)
			}
			i += n
			e.Ints = append(e.Ints, v)
		case 1: // fixed64
			if i+8 > len(data) {
				return nil, fmt.Errorf("short fixed64 at %d", i)
			}
			v := uint64(data[i]) | uint64(data[i+1])<<8 | uint64(data[i+2])<<16 |
				uint64(data[i+3])<<24 | uint64(data[i+4])<<32 | uint64(data[i+5])<<40 |
				uint64(data[i+6])<<48 | uint64(data[i+7])<<56
			i += 8
			e.Ints = append(e.Ints, v)
		case 5: // fixed32
			if i+4 > len(data) {
				return nil, fmt.Errorf("short fixed32 at %d", i)
			}
			v := uint64(data[i]) | uint64(data[i+1])<<8 | uint64(data[i+2])<<16 | uint64(data[i+3])<<24
			i += 4
			e.Ints = append(e.Ints, v)
		case 2: // length-delimited
			n, nn := varint(data[i:])
			if nn <= 0 {
				return nil, fmt.Errorf("bad length at %d", i)
			}
			i += nn
			if uint64(len(data)-i) < n {
				return nil, fmt.Errorf("short bytes at %d", i)
			}
			e.Bytes = append(e.Bytes, data[i:i+int(n)])
			i += int(n)
		default:
			return nil, fmt.Errorf("unknown wire type %d at %d", wt, i)
		}
	}
	return m, nil
}

func varint(b []byte) (uint64, int) {
	var v uint64
	var s uint
	for i, c := range b {
		v |= uint64(c&0x7F) << s
		s += 7
		if c&0x80 == 0 {
			return v, i + 1
		}
		if s >= 64 {
			return 0, -1
		}
	}
	return 0, -1
}

func appendVarint(b []byte, v uint64) []byte {
	for {
		c := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b = append(b, c|0x80)
		} else {
			return append(b, c)
		}
	}
}

// Enc is a protobuf message builder for the test fixtures.
type Enc struct{ b []byte }

// Bytes returns the encoded message.
func (e *Enc) Bytes() []byte { return e.b }

func (e *Enc) key(field uint32, wt int) {
	e.b = appendVarint(e.b, uint64(field<<3|uint32(wt)))
}

// Varint writes an int32/int64/uint32/bool/enum field.
func (e *Enc) Varint(field uint32, v uint64) {
	e.key(field, 0)
	e.b = appendVarint(e.b, v)
}

// Fixed64 writes a fixed64 field.
func (e *Enc) Fixed64(field uint32, v uint64) {
	e.key(field, 1)
	var b [8]byte
	for i := range b {
		b[i] = byte(v >> (8 * i))
	}
	e.b = append(e.b, b[:]...)
}

// Fixed32 writes a fixed32 field.
func (e *Enc) Fixed32(field uint32, v uint32) {
	e.key(field, 5)
	e.b = append(e.b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// BytesField writes a bytes/string/embedded field.
func (e *Enc) BytesField(field uint32, v []byte) {
	e.key(field, 2)
	e.b = appendVarint(e.b, uint64(len(v)))
	e.b = append(e.b, v...)
}

// Str writes a string field.
func (e *Enc) Str(field uint32, s string) { e.BytesField(field, []byte(s)) }

// MsgField writes an embedded message field.
func (e *Enc) MsgField(field uint32, m *Enc) { e.BytesField(field, m.b) }
