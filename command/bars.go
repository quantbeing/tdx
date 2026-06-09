package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

type BarsCommand struct {
	Market   model.Market
	Code     string
	Category model.KlineCategory
	Start    int
	Count    int
	Index    bool
}

func NewSecurityBarsCommand(market model.Market, code string, category model.KlineCategory, start int, count int) BarsCommand {
	return BarsCommand{Market: market, Code: code, Category: category, Start: start, Count: count}
}

func NewIndexBarsCommand(market model.Market, code string, category model.KlineCategory, start int, count int) BarsCommand {
	return BarsCommand{Market: market, Code: code, Category: category, Start: start, Count: count, Index: true}
}

func (c BarsCommand) Operation() string {
	if c.Index {
		return "index_bars"
	}
	return "security_bars"
}

func (c BarsCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	return binary.LittleEndian.AppendUint16(
		appendBarsRequestPrefix(code, c),
		0,
	), nil
}

func appendBarsRequestPrefix(code [6]byte, c BarsCommand) []byte {
	req := make([]byte, 0, 40)
	req = binary.LittleEndian.AppendUint16(req, 0x010c)
	req = binary.LittleEndian.AppendUint32(req, 0x01016408)
	req = binary.LittleEndian.AppendUint16(req, 0x001c)
	req = binary.LittleEndian.AppendUint16(req, 0x001c)
	req = binary.LittleEndian.AppendUint16(req, 0x052d)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Category))
	req = binary.LittleEndian.AppendUint16(req, 1)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Start))
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Count))
	req = binary.LittleEndian.AppendUint32(req, 0)
	req = binary.LittleEndian.AppendUint32(req, 0)
	return req
}

func (c BarsCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("%s header truncated: %d", c.Operation(), len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	out := make([]model.Bar, 0, count)
	preDiffBase := 0
	for i := 0; i < count; i++ {
		start := pos
		tm, next, err := codec.GetDateTime(int(c.Category), body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s datetime[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		openDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s open[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		closeDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s close[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		highDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s high[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		lowDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s low[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		vol, next, err := codec.GetVolume(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s vol[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		amount, next, err := codec.GetVolume(body, pos)
		if err != nil {
			return nil, fmt.Errorf("%s amount[%d]: %w", c.Operation(), i, err)
		}
		pos = next
		var up, down uint16
		if c.Index {
			if pos+4 > len(body) {
				return nil, fmt.Errorf("index_bars breadth[%d] truncated", i)
			}
			up = binary.LittleEndian.Uint16(body[pos : pos+2])
			down = binary.LittleEndian.Uint16(body[pos+2 : pos+4])
			pos += 4
		}
		openAbs := openDiff + preDiffBase
		closeAbs := openAbs + closeDiff
		highAbs := openAbs + highDiff
		lowAbs := openAbs + lowDiff
		preDiffBase = openAbs + closeDiff
		out = append(out, model.Bar{
			Market: c.Market, Code: c.Code, Category: c.Category,
			Year: tm.Year, Month: tm.Month, Day: tm.Day, Hour: tm.Hour, Minute: tm.Minute,
			Open: model.NewPriceFromMilli(openAbs), Close: model.NewPriceFromMilli(closeAbs),
			High: model.NewPriceFromMilli(highAbs), Low: model.NewPriceFromMilli(lowAbs),
			Vol: vol, Amount: amount, UpCount: up, DownCount: down,
			Raw: append([]byte(nil), body[start:pos]...),
		})
	}
	return out, nil
}
