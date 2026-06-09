package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

type HistoryFundFlowCommand struct {
	Market model.Market
	Code   string
	Start  int
	Count  int
}

func NewHistoryFundFlowCommand(market model.Market, code string, start int, count int) HistoryFundFlowCommand {
	return HistoryFundFlowCommand{Market: market, Code: code, Start: start, Count: count}
}

func (c HistoryFundFlowCommand) Operation() string { return "history_fund_flow" }

func (c HistoryFundFlowCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	req := make([]byte, 0, 40)
	req = binary.LittleEndian.AppendUint16(req, 0x010c)
	req = binary.LittleEndian.AppendUint32(req, 0x01016408)
	req = binary.LittleEndian.AppendUint16(req, 0x001c)
	req = binary.LittleEndian.AppendUint16(req, 0x001c)
	req = binary.LittleEndian.AppendUint16(req, 0x052d)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint16(req, 22)
	req = binary.LittleEndian.AppendUint16(req, 1)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Start))
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Count))
	req = binary.LittleEndian.AppendUint32(req, 0)
	req = binary.LittleEndian.AppendUint32(req, 0)
	req = binary.LittleEndian.AppendUint16(req, 0)
	return req, nil
}

func (c HistoryFundFlowCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 11 {
		return []model.HistoricalFundFlow{}, nil
	}
	count := int(binary.LittleEndian.Uint16(body[9:11]))
	pos := 11
	out := make([]model.HistoricalFundFlow, 0, count)
	for i := 0; i < count; i++ {
		start := pos
		if pos+36 > len(body) {
			return out, fmt.Errorf("history_fund_flow record[%d] truncated: offset=%d len=%d", i, pos, len(body))
		}
		rawDate := int(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4
		values := make([]float64, 8)
		for j := range values {
			var err error
			values[j], pos, err = codec.GetVolume(body, pos)
			if err != nil {
				return out, fmt.Errorf("history_fund_flow amount[%d][%d]: %w", i, j, err)
			}
		}
		out = append(out, model.HistoricalFundFlow{
			Year: rawDate / 10000, Month: (rawDate / 100) % 100, Day: rawDate % 100,
			SuperIn: values[0], LargeIn: values[1], MediumIn: values[2], SmallIn: values[3],
			SuperOut: values[4], LargeOut: values[5], MediumOut: values[6], SmallOut: values[7],
			Raw: append([]byte(nil), body[start:pos]...),
		})
	}
	return out, nil
}
