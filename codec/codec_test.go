package codec

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPriceVarintRoundTrip(t *testing.T) {
	values := []int{0, 1, 63, 64, 1000, -1, -63, -64, -1000, 22694}
	for _, value := range values {
		encoded := PutPrice(value)
		got, pos, err := GetPrice(encoded, 0)
		if err != nil {
			t.Fatalf("GetPrice(%d): %v", value, err)
		}
		if got != value || pos != len(encoded) {
			t.Fatalf("roundtrip %d = %d pos=%d len=%d bytes=%x", value, got, pos, len(encoded), encoded)
		}
	}
}

func TestDecodeVolumeMatchesTDXCustomFloat(t *testing.T) {
	raw := []byte{0x82, 0x02, 0xb0, 0x4a}
	got, pos, err := GetVolume(raw, 0)
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if pos != 4 {
		t.Fatalf("pos = %d, want 4", pos)
	}
	if math.Abs(got-5767489) > 1 {
		t.Fatalf("volume = %f, want about 5767489", got)
	}
}

func TestDateTimeCategoryFormats(t *testing.T) {
	day := make([]byte, 4)
	binary.LittleEndian.PutUint32(day, 20260609)
	ts, pos, err := GetDateTime(9, day, 0)
	if err != nil {
		t.Fatalf("GetDateTime day: %v", err)
	}
	if pos != 4 || ts.Year != 2026 || ts.Month != 6 || ts.Day != 9 || ts.Hour != 15 || ts.Minute != 0 {
		t.Fatalf("day datetime = %+v pos=%d", ts, pos)
	}

	minute := make([]byte, 4)
	zipday := uint16((2026-2004)<<11 + 6*100 + 9)
	binary.LittleEndian.PutUint16(minute[0:2], zipday)
	binary.LittleEndian.PutUint16(minute[2:4], 9*60+31)
	ts, pos, err = GetDateTime(0, minute, 0)
	if err != nil {
		t.Fatalf("GetDateTime minute: %v", err)
	}
	if pos != 4 || ts.Year != 2026 || ts.Month != 6 || ts.Day != 9 || ts.Hour != 9 || ts.Minute != 31 {
		t.Fatalf("minute datetime = %+v pos=%d", ts, pos)
	}
}
