package command

import (
	"encoding/binary"
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
