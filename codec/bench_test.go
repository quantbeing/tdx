package codec

import (
	"encoding/binary"
	"testing"
)

func BenchmarkGetPrice(b *testing.B) {
	data := PutPrice(22694)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := GetPrice(data, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPutPrice(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = PutPrice(22694)
	}
}

func BenchmarkGetVolume(b *testing.B) {
	data := []byte{0x82, 0x02, 0xb0, 0x4a}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := GetVolume(data, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetDateTimeMinute(b *testing.B) {
	data := make([]byte, 4)
	zipday := uint16((2026-2004)<<11 + 6*100 + 9)
	binary.LittleEndian.PutUint16(data[0:2], zipday)
	binary.LittleEndian.PutUint16(data[2:4], 9*60+31)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := GetDateTime(0, data, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetDateTimeDay(b *testing.B) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 20260609)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := GetDateTime(4, data, 0); err != nil {
			b.Fatal(err)
		}
	}
}
