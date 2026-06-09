package command

import (
	"encoding/binary"
	"testing"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

func TestSecurityListParserPreservesRawAndUnknownFields(t *testing.T) {
	body := make([]byte, 2)
	binary.LittleEndian.PutUint16(body, 1)
	row := make([]byte, 29)
	copy(row[0:6], []byte("600519"))
	binary.LittleEndian.PutUint16(row[6:8], 100)
	copy(row[8:16], []byte{0xb9, 0xf3, 0xd6, 0xdd, 0xc3, 0xa9, 0xcc, 0xa8}) // 贵州茅台 GBK, 8 bytes
	copy(row[16:20], []byte{1, 2, 3, 4})
	row[20] = 2
	binary.LittleEndian.PutUint32(row[21:25], 0xb04a0282)
	copy(row[25:29], []byte{5, 6, 7, 8})
	body = append(body, row...)

	reply, err := NewSecurityListCommand(model.MarketSH, 0).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	items := reply.([]model.Security)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	got := items[0]
	if got.Code != "600519" || got.Name != "贵州茅台" || got.VolUnit != 100 || got.DecimalPoint != 2 {
		t.Fatalf("security = %+v", got)
	}
	if len(got.Raw) != 29 || got.Unknown1 != [4]byte{1, 2, 3, 4} || got.Unknown2 != [4]byte{5, 6, 7, 8} {
		t.Fatalf("raw/unknown not preserved: %+v", got)
	}
}

func TestSecurityBarsAndIndexBarsParseDifferentRecordShapes(t *testing.T) {
	stockBody := buildBarsBody(t, false)
	stockReply, err := NewSecurityBarsCommand(model.MarketSH, "600519", model.KlineDay, 0, 1).ParseResponse(stockBody)
	if err != nil {
		t.Fatalf("stock ParseResponse: %v", err)
	}
	stockBars := stockReply.([]model.Bar)
	if len(stockBars) != 1 || stockBars[0].Close.String() != "10.5" || stockBars[0].UpCount != 0 {
		t.Fatalf("stock bars = %+v", stockBars)
	}

	indexBody := buildBarsBody(t, true)
	indexReply, err := NewIndexBarsCommand(model.MarketSH, "000001", model.KlineDay, 0, 1).ParseResponse(indexBody)
	if err != nil {
		t.Fatalf("index ParseResponse: %v", err)
	}
	indexBars := indexReply.([]model.Bar)
	if len(indexBars) != 1 || indexBars[0].UpCount != 123 || indexBars[0].DownCount != 456 {
		t.Fatalf("index bars = %+v", indexBars)
	}
}

func buildBarsBody(t *testing.T, withBreadth bool) []byte {
	t.Helper()
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint32(body, 20260609)
	body = append(body, codec.PutPrice(10000)...)
	body = append(body, codec.PutPrice(500)...)
	body = append(body, codec.PutPrice(800)...)
	body = append(body, codec.PutPrice(-200)...)
	body = binary.LittleEndian.AppendUint32(body, 0xb04a0282)
	body = binary.LittleEndian.AppendUint32(body, 0xb04a0282)
	if withBreadth {
		body = binary.LittleEndian.AppendUint16(body, 123)
		body = binary.LittleEndian.AppendUint16(body, 456)
	}
	return body
}
