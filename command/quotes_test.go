package command

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

func TestSecurityQuotesParserPreservesFiveLevelsAndUnknownFields(t *testing.T) {
	body := buildQuoteBody(t)
	reply, err := NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "600519"}}).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	quotes := reply.([]model.Quote)
	if len(quotes) != 1 {
		t.Fatalf("len(quotes) = %d, want 1", len(quotes))
	}
	q := quotes[0]
	if q.Market != model.MarketSH || q.Code != "600519" {
		t.Fatalf("identity = %s %s", q.Market, q.Code)
	}
	if q.Price.String() != "10.5" || q.PreClose.String() != "10" || q.Open.String() != "10.3" ||
		q.High.String() != "10.6" || q.Low.String() != "9.7" {
		t.Fatalf("prices = price=%s pre=%s open=%s high=%s low=%s", q.Price, q.PreClose, q.Open, q.High, q.Low)
	}
	if q.Bid[0].Price.String() != "10.49" || q.Ask[0].Price.String() != "10.51" ||
		q.Bid[4].Volume != 50 || q.Ask[4].Volume != 60 {
		t.Fatalf("levels = bid=%+v ask=%+v", q.Bid, q.Ask)
	}
	if q.Unknown2 != -1 || q.Unknown3 != 22694 || q.Unknown5 != 1 || q.Unknown8 != 4 || q.ServerTime != "14:59:57.163" {
		t.Fatalf("unknown/server fields = %+v", q)
	}
	if len(q.Raw) == 0 {
		t.Fatal("Raw is empty")
	}
}

func TestSecurityQuotesParserKeepsOffsetAcrossRecords(t *testing.T) {
	body, err := hex.DecodeString("01130200013630303531398610a0aa0fba0abb0abc0ad905a1dc970ee0aa0f94b303ba07c6a6504fa1fa01b3b801ec0180a7314100040e480101014c320105543c0103573e01011603000000000000861000303030303031331299114a4c0651a8f5a60ed911a8ab8c01ad8301af3e984e9fd93d89d24e00b189040001be66af3a4102af2aa2444203a249a233430498308c694405ae1e90b40196080000000000003312")
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	reply, err := NewSecurityQuotesCommand([]model.Symbol{
		{Market: model.MarketSH, Code: "600519"},
		{Market: model.MarketSZ, Code: "000001"},
	}).ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	quotes := reply.([]model.Quote)
	if len(quotes) != 2 {
		t.Fatalf("len(quotes) = %d, want 2", len(quotes))
	}
	if quotes[1].Market != model.MarketSZ || quotes[1].Code != "000001" {
		t.Fatalf("second quote = %+v", quotes[1])
	}
	if len(quotes[1].Raw) == 0 || quotes[1].Raw[0] != byte(model.MarketSZ) {
		t.Fatalf("second raw starts with %x", quotes[1].Raw[:minInt(len(quotes[1].Raw), 8)])
	}
}

func buildQuoteBody(t *testing.T) []byte {
	t.Helper()
	body := []byte{0xb1, 0xcb}
	body = binary.LittleEndian.AppendUint16(body, 1)
	recordStart := len(body)
	body = append(body, byte(model.MarketSH))
	body = append(body, []byte("600519")...)
	body = binary.LittleEndian.AppendUint16(body, 11)
	for _, v := range []int{1050, -50, -20, 10, -80, 14999212, -1050, 10000, 100} {
		body = append(body, codec.PutPrice(v)...)
	}
	body = binary.LittleEndian.AppendUint32(body, 0x4ab00282)
	for _, v := range []int{400, 600, -1, 22694} {
		body = append(body, codec.PutPrice(v)...)
	}
	for i := 1; i <= 5; i++ {
		body = append(body, codec.PutPrice(-i)...)
		body = append(body, codec.PutPrice(i)...)
		body = append(body, codec.PutPrice(i*10)...)
		body = append(body, codec.PutPrice(i*12)...)
	}
	body = binary.LittleEndian.AppendUint16(body, 7)
	for _, v := range []int{1, 2, 3, 4} {
		body = append(body, codec.PutPrice(v)...)
	}
	body = binary.LittleEndian.AppendUint16(body, uint16(int16(25)))
	body = binary.LittleEndian.AppendUint16(body, 9)
	if len(body[recordStart:]) == 0 {
		t.Fatal("empty quote record")
	}
	return body
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
