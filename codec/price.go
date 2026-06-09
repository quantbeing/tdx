// Package codec implements low-level TDX field encoders and decoders.
package codec

import "fmt"

func GetPrice(data []byte, pos int) (int, int, error) {
	start := pos
	if pos >= len(data) {
		return 0, pos, fmt.Errorf("tdx price varint truncated at offset %d", start)
	}
	shift := 6
	b := data[pos]
	value := int(b & 0x3f)
	negative := b&0x40 != 0
	if b&0x80 != 0 {
		for {
			pos++
			if pos >= len(data) {
				return 0, pos, fmt.Errorf("tdx price varint truncated at offset %d", start)
			}
			b = data[pos]
			value |= int(b&0x7f) << shift
			shift += 7
			if b&0x80 == 0 {
				break
			}
		}
	}
	pos++
	if negative {
		value = -value
	}
	return value, pos, nil
}

func PutPrice(value int) []byte {
	negative := value < 0
	if negative {
		value = -value
	}
	first := byte(value & 0x3f)
	value >>= 6
	if negative {
		first |= 0x40
	}
	if value != 0 {
		first |= 0x80
	}
	out := []byte{first}
	for value != 0 {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}
