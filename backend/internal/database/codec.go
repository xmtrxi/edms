package database

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// The SurrealDB Go SDK encodes time.Time as CBOR tag 0 (an RFC 3339 string).
// SurrealDB 4 rejects that and only accepts tag 12 ([seconds, nanoseconds]),
// which SurrealDB 2 and 3 accept as well. datetimeMarshaler wraps the SDK's
// marshaler and rewrites every tag-0 datetime in the encoded request into the
// tag-12 form, so the app works against all of those server versions.

const (
	cborTagDatetimeString  = 0
	cborTagDatetimeCompact = 12
)

type marshaler interface {
	Marshal(v any) ([]byte, error)
}

type datetimeMarshaler struct {
	inner marshaler
}

func (m datetimeMarshaler) Marshal(v any) ([]byte, error) {
	data, err := m.inner.Marshal(v)
	if err != nil {
		return nil, err
	}
	return rewriteDatetimeTags(data)
}

// rewriteDatetimeTags copies a sequence of CBOR items, replacing tag 0
// datetimes with tag 12 datetimes.
func rewriteDatetimeTags(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data)+16)
	for pos := 0; pos < len(data); {
		next, rewritten, err := rewriteItem(data, pos, out)
		if err != nil {
			return nil, err
		}
		pos, out = next, rewritten
	}
	return out, nil
}

var errTruncatedCBOR = errors.New("cbor: truncated data")

// readHeader decodes the item header at pos and returns its major type,
// argument, whether it is indefinite-length, and the position after the header.
func readHeader(data []byte, pos int) (major byte, arg uint64, indefinite bool, next int, err error) {
	if pos >= len(data) {
		return 0, 0, false, 0, errTruncatedCBOR
	}
	ib := data[pos]
	major, ai := ib>>5, ib&0x1f
	pos++
	switch {
	case ai < 24:
		return major, uint64(ai), false, pos, nil
	case ai == 31:
		return major, 0, true, pos, nil
	case ai > 27:
		return 0, 0, false, 0, fmt.Errorf("cbor: invalid additional info %d", ai)
	}
	size := 1 << (ai - 24)
	if pos+size > len(data) {
		return 0, 0, false, 0, errTruncatedCBOR
	}
	switch size {
	case 1:
		arg = uint64(data[pos])
	case 2:
		arg = uint64(binary.BigEndian.Uint16(data[pos:]))
	case 4:
		arg = uint64(binary.BigEndian.Uint32(data[pos:]))
	case 8:
		arg = binary.BigEndian.Uint64(data[pos:])
	}
	return major, arg, false, pos + size, nil
}

// rewriteItem appends the (possibly rewritten) item starting at pos to out and
// returns the position after it.
func rewriteItem(data []byte, pos int, out []byte) (int, []byte, error) {
	start := pos
	major, arg, indefinite, pos, err := readHeader(data, pos)
	if err != nil {
		return 0, nil, err
	}

	switch major {
	case 0, 1: // unsigned / negative integer
		return pos, append(out, data[start:pos]...), nil

	case 2, 3: // byte / text string
		if !indefinite {
			end := pos + int(arg)
			if arg > uint64(len(data)) || end > len(data) {
				return 0, nil, errTruncatedCBOR
			}
			return end, append(out, data[start:end]...), nil
		}
		out = append(out, data[start:pos]...)
		for {
			if pos >= len(data) {
				return 0, nil, errTruncatedCBOR
			}
			if data[pos] == 0xff {
				return pos + 1, append(out, 0xff), nil
			}
			if pos, out, err = rewriteItem(data, pos, out); err != nil {
				return 0, nil, err
			}
		}

	case 4, 5: // array / map
		out = append(out, data[start:pos]...)
		if indefinite {
			for {
				if pos >= len(data) {
					return 0, nil, errTruncatedCBOR
				}
				if data[pos] == 0xff {
					return pos + 1, append(out, 0xff), nil
				}
				if pos, out, err = rewriteItem(data, pos, out); err != nil {
					return 0, nil, err
				}
			}
		}
		count := arg
		if major == 5 {
			count *= 2
		}
		for i := uint64(0); i < count; i++ {
			if pos, out, err = rewriteItem(data, pos, out); err != nil {
				return 0, nil, err
			}
		}
		return pos, out, nil

	case 6: // tag
		if arg == cborTagDatetimeString {
			var s string
			contentStart := pos
			next, skipped, err := rewriteItem(data, pos, nil)
			if err != nil {
				return 0, nil, err
			}
			if err := cbor.Unmarshal(skipped, &s); err == nil {
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
					enc, err := cbor.Marshal(cbor.Tag{
						Number:  cborTagDatetimeCompact,
						Content: [2]int64{t.Unix(), int64(t.Nanosecond())},
					})
					if err != nil {
						return 0, nil, err
					}
					return next, append(out, enc...), nil
				}
			}
			out = append(out, data[start:contentStart]...)
			return next, append(out, skipped...), nil
		}
		out = append(out, data[start:pos]...)
		return rewriteItem(data, pos, out)

	default: // 7: simple values and floats
		return pos, append(out, data[start:pos]...), nil
	}
}
