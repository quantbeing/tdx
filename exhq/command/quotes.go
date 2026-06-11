package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/exhq/model"
)

const (
	singleQuoteSize = 150
	quoteListSize   = 300
)

type InstrumentQuoteCommand struct {
	Market model.MarketID
	Code   string
}

func NewInstrumentQuoteCommand(market model.MarketID, code string) InstrumentQuoteCommand {
	return InstrumentQuoteCommand{Market: market, Code: code}
}

func (c InstrumentQuoteCommand) Operation() string { return "exhq_instrument_quote" }

func (c InstrumentQuoteCommand) BuildRequest() ([]byte, error) {
	req := mustHex("0101080202010c000c00fa23")
	req = append(req, byte(c.Market))
	req = appendCode(req, c.Code, 9)
	return req, nil
}

func (c InstrumentQuoteCommand) ParseResponse(body []byte) (any, error) {
	return ParseInstrumentQuote(body)
}

func ParseInstrumentQuote(body []byte) (model.Quote, error) {
	if err := requireLen("exhq_instrument_quote", body, singleQuoteSize); err != nil {
		return model.Quote{}, err
	}
	raw := append([]byte(nil), body[:singleQuoteSize]...)
	q := parseSingleQuoteRaw(raw)
	q.Raw = raw
	return q, nil
}

func parseSingleQuoteRaw(raw []byte) model.Quote {
	q := model.Quote{
		Market:  model.MarketID(raw[0]),
		Code:    trimCode(raw[1:10]),
		Unknown: append([]byte(nil), raw[10:14]...),
	}
	pos := 14
	q.PreClose = float32At(raw[pos : pos+4])
	q.Open = float32At(raw[pos+4 : pos+8])
	q.High = float32At(raw[pos+8 : pos+12])
	q.Low = float32At(raw[pos+12 : pos+16])
	q.Price = float32At(raw[pos+16 : pos+20])
	pos += 20
	q.OpenVolume = binary.LittleEndian.Uint32(raw[pos : pos+4])
	q.Unknown1 = binary.LittleEndian.Uint32(raw[pos+4 : pos+8])
	q.TotalVolume = binary.LittleEndian.Uint32(raw[pos+8 : pos+12])
	q.CurrentVolume = binary.LittleEndian.Uint32(raw[pos+12 : pos+16])
	q.Unknown2 = binary.LittleEndian.Uint32(raw[pos+16 : pos+20])
	q.InnerVolume = binary.LittleEndian.Uint32(raw[pos+20 : pos+24])
	q.OuterVolume = binary.LittleEndian.Uint32(raw[pos+24 : pos+28])
	q.Unknown3 = binary.LittleEndian.Uint32(raw[pos+28 : pos+32])
	q.OpenInterest = binary.LittleEndian.Uint32(raw[pos+32 : pos+36])
	pos += 36
	for i := 0; i < 5; i++ {
		q.Bid[i].Price = float32At(raw[pos+i*4 : pos+i*4+4])
	}
	pos += 20
	for i := 0; i < 5; i++ {
		q.Bid[i].Volume = binary.LittleEndian.Uint32(raw[pos+i*4 : pos+i*4+4])
	}
	pos += 20
	for i := 0; i < 5; i++ {
		q.Ask[i].Price = float32At(raw[pos+i*4 : pos+i*4+4])
	}
	pos += 20
	for i := 0; i < 5; i++ {
		q.Ask[i].Volume = binary.LittleEndian.Uint32(raw[pos+i*4 : pos+i*4+4])
	}
	return q
}

type InstrumentQuoteListCommand struct {
	Market   model.MarketID
	Category uint8
	Start    int
	Count    int
}

func NewInstrumentQuoteListCommand(market model.MarketID, category uint8, start, count int) InstrumentQuoteListCommand {
	return InstrumentQuoteListCommand{Market: market, Category: category, Start: start, Count: count}
}

func (c InstrumentQuoteListCommand) Operation() string { return "exhq_instrument_quote_list" }

func (c InstrumentQuoteListCommand) BuildRequest() ([]byte, error) {
	req := mustHex("01c1060b00020b000b000024")
	req = append(req, byte(c.Market))
	req = putUint16(req, 0)
	req = putUint16(req, c.Start)
	req = putUint16(req, c.Count)
	req = putUint16(req, 1)
	return req, nil
}

func (c InstrumentQuoteListCommand) ParseResponse(body []byte) (any, error) {
	return ParseInstrumentQuoteList(body, c.Category)
}

func ParseInstrumentQuoteList(body []byte, category uint8) ([]model.Quote, error) {
	if err := requireLen("exhq_instrument_quote_list", body, 2); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint16(body[0:2]))
	pos := 2
	out := make([]model.Quote, 0, count)
	for i := 0; i < count; i++ {
		if pos+quoteListSize > len(body) {
			return nil, fmt.Errorf("exhq_instrument_quote_list record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+quoteListSize]...)
		pos += quoteListSize
		q := model.Quote{
			Market:  model.MarketID(raw[0]),
			Code:    trimCode(raw[1:10]),
			Unknown: append([]byte(nil), raw[150:300]...),
			Raw:     raw,
		}
		payload := raw[10:]
		switch category {
		case 2:
			parseHKQuoteListPayload(payload, &q)
		case 3:
			parseFuturesQuoteListPayload(payload, &q)
		default:
			parseFuturesQuoteListPayload(payload, &q)
		}
		out = append(out, q)
	}
	return out, nil
}

func parseFuturesQuoteListPayload(payload []byte, q *model.Quote) {
	q.Unknown1 = binary.LittleEndian.Uint32(payload[0:4])
	q.PreClose = float32At(payload[4:8])
	q.Open = float32At(payload[8:12])
	q.High = float32At(payload[12:16])
	q.Low = float32At(payload[16:20])
	q.Price = float32At(payload[20:24])
	q.OpenVolume = binary.LittleEndian.Uint32(payload[24:28])
	q.TotalVolume = binary.LittleEndian.Uint32(payload[32:36])
	q.CurrentVolume = binary.LittleEndian.Uint32(payload[36:40])
	q.Amount = float32At(payload[40:44])
	q.InnerVolume = binary.LittleEndian.Uint32(payload[44:48])
	q.OuterVolume = binary.LittleEndian.Uint32(payload[48:52])
	q.OpenInterest = binary.LittleEndian.Uint32(payload[56:60])
	q.Bid[0].Price = float32At(payload[60:64])
	q.Bid[0].Volume = binary.LittleEndian.Uint32(payload[80:84])
	q.Ask[0].Price = float32At(payload[100:104])
	q.Ask[0].Volume = binary.LittleEndian.Uint32(payload[120:124])
}

func parseHKQuoteListPayload(payload []byte, q *model.Quote) {
	q.Unknown1 = binary.LittleEndian.Uint32(payload[0:4])
	q.PreClose = float32At(payload[4:8])
	q.Open = float32At(payload[8:12])
	q.High = float32At(payload[12:16])
	q.Low = float32At(payload[16:20])
	q.Price = float32At(payload[20:24])
	q.Bid[0].Price = float32At(payload[28:32])
	q.TotalVolume = binary.LittleEndian.Uint32(payload[32:36])
	q.CurrentVolume = binary.LittleEndian.Uint32(payload[36:40])
	q.Amount = float32At(payload[40:44])
	q.InnerVolume = binary.LittleEndian.Uint32(payload[52:56])
	q.OuterVolume = binary.LittleEndian.Uint32(payload[56:60])
	for i := 0; i < 5; i++ {
		q.Bid[i].Price = float32At(payload[60+i*4 : 64+i*4])
		q.Bid[i].Volume = binary.LittleEndian.Uint32(payload[80+i*4 : 84+i*4])
		q.Ask[i].Price = float32At(payload[100+i*4 : 104+i*4])
		q.Ask[i].Volume = binary.LittleEndian.Uint32(payload[120+i*4 : 124+i*4])
	}
}
