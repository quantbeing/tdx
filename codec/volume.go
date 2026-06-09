package codec

import (
	"encoding/binary"
	"fmt"
	"math"
)

func GetVolume(data []byte, pos int) (float64, int, error) {
	if pos+4 > len(data) {
		return 0, pos, fmt.Errorf("tdx volume truncated at offset %d", pos)
	}
	raw := binary.LittleEndian.Uint32(data[pos : pos+4])
	return DecodeVolume(raw), pos + 4, nil
}

func DecodeVolume(raw uint32) float64 {
	if raw == 0 {
		return 0
	}
	logpoint := int((raw >> 24) & 0xff)
	hleax := int((raw >> 16) & 0xff)
	lheax := int((raw >> 8) & 0xff)
	lleax := int(raw & 0xff)

	base := pow2(logpoint*2 - 0x7f)
	expH := logpoint*2 - 0x86
	var hi float64
	if hleax > 0x80 {
		hi = pow2(expH)*128 + float64(hleax&0x7f)*pow2(expH+1)
	} else {
		hi = pow2(expH) * float64(hleax)
	}
	mid := pow2(logpoint*2-0x8e) * float64(lheax)
	lo := pow2(logpoint*2-0x96) * float64(lleax)
	if hleax&0x80 != 0 {
		mid *= 2
		lo *= 2
	}
	return base + hi + mid + lo
}

func pow2(exp int) float64 {
	return math.Ldexp(1, exp)
}
