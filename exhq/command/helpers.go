package command

import (
	"encoding/binary"
	"math"
	"strings"

	"github.com/quantbeing/tdx/codec"
)

func appendCode(dst []byte, code string, n int) []byte {
	buf := make([]byte, n)
	copy(buf, []byte(code))
	return append(dst, buf...)
}

func trimCode(raw []byte) string {
	return strings.TrimRight(string(raw), "\x00 ")
}

func decodeText(raw []byte) string {
	return codec.DecodeGBKBestEffort(raw)
}

func float32At(b []byte) float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
}

func putUint16(dst []byte, v int) []byte {
	return binary.LittleEndian.AppendUint16(dst, uint16(v))
}

func putUint32(dst []byte, v int) []byte {
	return binary.LittleEndian.AppendUint32(dst, uint32(v))
}
