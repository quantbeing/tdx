package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/exhq/model"
)

type InstrumentBarsCommand struct {
	Category model.KlineCategory
	Market   model.MarketID
	Code     string
	Start    int
	Count    int
}

func NewInstrumentBarsCommand(category model.KlineCategory, market model.MarketID, code string, start, count int) InstrumentBarsCommand {
	return InstrumentBarsCommand{Category: category, Market: market, Code: code, Start: start, Count: count}
}

func (c InstrumentBarsCommand) Operation() string { return "exhq_instrument_bars" }

func (c InstrumentBarsCommand) BuildRequest() ([]byte, error) {
	req := mustHex("0101086a010116001600ff23")
	req = append(req, byte(c.Market))
	req = appendCode(req, c.Code, 9)
	req = putUint16(req, int(c.Category))
	req = putUint16(req, 1)
	req = putUint32(req, c.Start)
	req = putUint16(req, c.Count)
	return req, nil
}

func (c InstrumentBarsCommand) ParseResponse(body []byte) (any, error) {
	rows, err := ParseInstrumentBars(body, c.Category)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Market = c.Market
		rows[i].Code = c.Code
		rows[i].Category = c.Category
	}
	return rows, nil
}

func ParseInstrumentBars(body []byte, category model.KlineCategory) ([]model.Bar, error) {
	if err := requireLen("exhq_instrument_bars", body, 20); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint16(body[18:20]))
	pos := 20
	out := make([]model.Bar, 0, count)
	for i := 0; i < count; i++ {
		if pos+32 > len(body) {
			return nil, fmt.Errorf("exhq_instrument_bars record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+32]...)
		tm, _, err := codec.GetDateTime(int(category), body, pos)
		if err != nil {
			return nil, fmt.Errorf("exhq_instrument_bars datetime[%d]: %w", i, err)
		}
		pos += 4
		bar := parseBarTail(body[pos:pos+28], raw)
		bar.Category = category
		bar.Year = tm.Year
		bar.Month = tm.Month
		bar.Day = tm.Day
		bar.Hour = tm.Hour
		bar.Minute = tm.Minute
		out = append(out, bar)
		pos += 28
	}
	return out, nil
}

type HistoryInstrumentBarsRangeCommand struct {
	Market model.MarketID
	Code   string
	Start  int
	End    int
}

func NewHistoryInstrumentBarsRangeCommand(market model.MarketID, code string, start, end int) HistoryInstrumentBarsRangeCommand {
	return HistoryInstrumentBarsRangeCommand{Market: market, Code: code, Start: start, End: end}
}

func (c HistoryInstrumentBarsRangeCommand) Operation() string {
	return "exhq_history_instrument_bars_range"
}

func (c HistoryInstrumentBarsRangeCommand) BuildRequest() ([]byte, error) {
	req := mustHex("010138920001160016000d24")
	req = append(req, byte(c.Market))
	req = appendCode(req, c.Code, 9)
	req = putUint16(req, 7)
	req = putUint32(req, c.Start)
	req = putUint32(req, c.End)
	return req, nil
}

func (c HistoryInstrumentBarsRangeCommand) ParseResponse(body []byte) (any, error) {
	rows, err := ParseHistoryInstrumentBarsRange(body)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Market = c.Market
		rows[i].Code = c.Code
	}
	return rows, nil
}

func ParseHistoryInstrumentBarsRange(body []byte) ([]model.Bar, error) {
	if err := requireLen("exhq_history_instrument_bars_range", body, 14); err != nil {
		return nil, err
	}
	count := int(binary.LittleEndian.Uint16(body[12:14]))
	pos := 14
	out := make([]model.Bar, 0, count)
	for i := 0; i < count; i++ {
		if pos+32 > len(body) {
			return nil, fmt.Errorf("exhq_history_instrument_bars_range record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+32]...)
		dateRaw := binary.LittleEndian.Uint16(raw[0:2])
		timeRaw := binary.LittleEndian.Uint16(raw[2:4])
		bar := parseBarTail(raw[4:32], raw)
		bar.Year, bar.Month, bar.Day = parseExDate(dateRaw)
		bar.Hour = int(timeRaw / 60)
		bar.Minute = int(timeRaw % 60)
		bar.SettlementPrice = bar.Price
		out = append(out, bar)
		pos += 32
	}
	return out, nil
}

func parseBarTail(data []byte, raw []byte) model.Bar {
	return model.Bar{
		Open:     float32At(data[0:4]),
		High:     float32At(data[4:8]),
		Low:      float32At(data[8:12]),
		Close:    float32At(data[12:16]),
		Position: binary.LittleEndian.Uint32(data[16:20]),
		Trade:    binary.LittleEndian.Uint32(data[20:24]),
		Price:    float32At(data[24:28]),
		Raw:      raw,
	}
}

func parseExDate(v uint16) (year, month, day int) {
	n := int(v)
	return n/2048 + 2004, (n % 2048) / 100, (n % 2048) % 100
}
