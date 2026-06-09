package command

import (
	"encoding/binary"
	"testing"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

func TestMinuteTimeParserPreservesUnknown1(t *testing.T) {
	body := buildMinuteBody(false)
	reply, err := NewMinuteTimeDataCommand(model.MarketSH, "600519").ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	bars := reply.([]model.MinuteTime)
	if len(bars) != 2 {
		t.Fatalf("len(bars) = %d, want 2", len(bars))
	}
	if bars[0].Price.String() != "10" || bars[1].Price.String() != "10.5" {
		t.Fatalf("prices = %s %s, want 10 10.5", bars[0].Price, bars[1].Price)
	}
	if bars[0].Unknown1 != 77 || bars[1].Unknown1 != 88 || len(bars[0].Raw) == 0 {
		t.Fatalf("unknown/raw = %+v %+v", bars[0], bars[1])
	}
}

func TestHistoryMinuteTimeParserSkipsHistoryPrefix(t *testing.T) {
	body := buildMinuteBody(true)
	reply, err := NewHistoryMinuteTimeDataCommand(model.MarketSH, "600519", 20260609).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	bars := reply.([]model.MinuteTime)
	if len(bars) != 2 || bars[1].Unknown1 != 88 {
		t.Fatalf("bars = %+v", bars)
	}
}

func TestMinuteTimeParserSkipsLiveSymbolPrefix(t *testing.T) {
	body := make([]byte, 65)
	binary.LittleEndian.PutUint16(body[0:2], 2)
	body[4] = byte(model.MarketSH)
	copy(body[5:11], []byte("600519"))
	for _, rec := range [][3]int{{125980, 0, 1134}, {-180, 0, 502}} {
		body = append(body, codec.PutPrice(rec[0])...)
		body = append(body, codec.PutPrice(rec[1])...)
		body = append(body, codec.PutPrice(rec[2])...)
	}

	reply, err := NewMinuteTimeDataCommand(model.MarketSH, "600519").ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	rows := reply.([]model.MinuteTime)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Price.String() != "1259.8" || rows[0].Volume != 1134 ||
		rows[1].Price.String() != "1258" || rows[1].Volume != 502 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestTransactionParserPreservesNumOrdersAndUnknownLast(t *testing.T) {
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint16(body, 14*60+56)
	for _, v := range []int{1050, 200, 3, 1, 99} {
		body = append(body, codec.PutPrice(v)...)
	}

	reply, err := NewTransactionDataCommand(model.MarketSH, "600519", 0, 800).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	rows := reply.([]model.Transaction)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Hour != 14 || row.Minute != 56 || row.Price.String() != "10.5" || row.Vol != 200 ||
		row.NumOrders != 3 || row.BuyOrSell != 1 || row.UnknownLast != 99 || len(row.Raw) == 0 {
		t.Fatalf("row = %+v", row)
	}
}

func TestHistoryTransactionParserPreservesUnknownLastWithoutNumOrders(t *testing.T) {
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = binary.LittleEndian.AppendUint32(body, 0)
	body = binary.LittleEndian.AppendUint16(body, 9*60+31)
	for _, v := range []int{1234, 88, 2, 7} {
		body = append(body, codec.PutPrice(v)...)
	}

	reply, err := NewHistoryTransactionDataCommand(model.MarketSH, "600519", 20260609, 0, 800).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	rows := reply.([]model.Transaction)
	if len(rows) != 1 || rows[0].NumOrders != 0 || rows[0].UnknownLast != 7 || rows[0].Price.String() != "12.34" {
		t.Fatalf("rows = %+v", rows)
	}
}

func buildMinuteBody(history bool) []byte {
	body := make([]byte, 0)
	body = binary.LittleEndian.AppendUint16(body, 2)
	if history {
		body = binary.LittleEndian.AppendUint32(body, 0)
	} else {
		body = binary.LittleEndian.AppendUint16(body, 0)
	}
	for _, rec := range [][3]int{{1000, 77, 100}, {50, 88, 120}} {
		body = append(body, codec.PutPrice(rec[0])...)
		body = append(body, codec.PutPrice(rec[1])...)
		body = append(body, codec.PutPrice(rec[2])...)
	}
	return body
}
