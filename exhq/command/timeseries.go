package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/exhq/model"
)

type MinuteTimeDataCommand struct {
	Market  model.MarketID
	Code    string
	Date    int
	History bool
}

func NewMinuteTimeDataCommand(market model.MarketID, code string) MinuteTimeDataCommand {
	return MinuteTimeDataCommand{Market: market, Code: code}
}

func NewHistoryMinuteTimeDataCommand(market model.MarketID, code string, date int) MinuteTimeDataCommand {
	return MinuteTimeDataCommand{Market: market, Code: code, Date: date, History: true}
}

func (c MinuteTimeDataCommand) Operation() string {
	if c.History {
		return "exhq_history_minute_time"
	}
	return "exhq_minute_time"
}

func (c MinuteTimeDataCommand) BuildRequest() ([]byte, error) {
	if c.History {
		req := mustHex("010130000101100010000c24")
		req = putUint32(req, c.Date)
		req = append(req, byte(c.Market))
		req = appendCode(req, c.Code, 9)
		return req, nil
	}
	req := mustHex("0107080001010c000c000b24")
	req = append(req, byte(c.Market))
	req = appendCode(req, c.Code, 9)
	return req, nil
}

func (c MinuteTimeDataCommand) ParseResponse(body []byte) (any, error) {
	if c.History {
		return ParseHistoryMinuteTimeData(body, c.Date)
	}
	return ParseMinuteTimeData(body)
}

func ParseMinuteTimeData(body []byte) ([]model.MinuteTime, error) {
	return parseMinuteBody(body, 12, 0)
}

func ParseHistoryMinuteTimeData(body []byte, date int) ([]model.MinuteTime, error) {
	return parseMinuteBody(body, 20, date)
}

func parseMinuteBody(body []byte, headerLen int, date int) ([]model.MinuteTime, error) {
	if err := requireLen("exhq_minute_time", body, headerLen); err != nil {
		return nil, err
	}
	market := model.MarketID(body[0])
	code := trimCode(body[1:10])
	unknown := []byte(nil)
	if headerLen == 20 {
		unknown = append([]byte(nil), body[10:18]...)
	}
	count := int(binary.LittleEndian.Uint16(body[headerLen-2 : headerLen]))
	pos := headerLen
	out := make([]model.MinuteTime, 0, count)
	for i := 0; i < count; i++ {
		if pos+18 > len(body) {
			return nil, fmt.Errorf("exhq_minute_time record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+18]...)
		timeRaw := binary.LittleEndian.Uint16(raw[0:2])
		out = append(out, model.MinuteTime{
			Market:       market,
			Code:         code,
			Date:         date,
			Hour:         int(timeRaw / 60),
			Minute:       int(timeRaw % 60),
			Price:        float32At(raw[2:6]),
			AvgPrice:     float32At(raw[6:10]),
			Volume:       binary.LittleEndian.Uint32(raw[10:14]),
			OpenInterest: binary.LittleEndian.Uint32(raw[14:18]),
			Unknown:      append([]byte(nil), unknown...),
			Raw:          raw,
		})
		pos += 18
	}
	return out, nil
}

type TransactionDataCommand struct {
	Market  model.MarketID
	Code    string
	Date    int
	Start   int
	Count   int
	History bool
}

func NewTransactionDataCommand(market model.MarketID, code string, start, count int) TransactionDataCommand {
	return TransactionDataCommand{Market: market, Code: code, Start: start, Count: count}
}

func NewHistoryTransactionDataCommand(market model.MarketID, code string, date, start, count int) TransactionDataCommand {
	return TransactionDataCommand{Market: market, Code: code, Date: date, Start: start, Count: count, History: true}
}

func (c TransactionDataCommand) Operation() string {
	if c.History {
		return "exhq_history_transaction"
	}
	return "exhq_transaction"
}

func (c TransactionDataCommand) BuildRequest() ([]byte, error) {
	if c.History {
		req := mustHex("010130000201160016000624")
		req = putUint32(req, c.Date)
		req = append(req, byte(c.Market))
		req = appendCode(req, c.Code, 9)
		req = binary.LittleEndian.AppendUint32(req, uint32(int32(c.Start)))
		req = putUint16(req, c.Count)
		return req, nil
	}
	req := mustHex("01010800030112001200fc23")
	req = append(req, byte(c.Market))
	req = appendCode(req, c.Code, 9)
	req = binary.LittleEndian.AppendUint32(req, uint32(int32(c.Start)))
	req = putUint16(req, c.Count)
	return req, nil
}

func (c TransactionDataCommand) ParseResponse(body []byte) (any, error) {
	if c.History {
		return ParseHistoryTransactionData(body, c.Date)
	}
	return ParseTransactionData(body, 0)
}

func ParseTransactionData(body []byte, date int) ([]model.Transaction, error) {
	return parseTransactionBody(body, date)
}

func ParseHistoryTransactionData(body []byte, date int) ([]model.Transaction, error) {
	return parseTransactionBody(body, date)
}

func parseTransactionBody(body []byte, date int) ([]model.Transaction, error) {
	if err := requireLen("exhq_transaction", body, 16); err != nil {
		return nil, err
	}
	market := model.MarketID(body[0])
	code := trimCode(body[1:10])
	unknown := append([]byte(nil), body[10:14]...)
	count := int(binary.LittleEndian.Uint16(body[14:16]))
	pos := 16
	out := make([]model.Transaction, 0, count)
	for i := 0; i < count; i++ {
		if pos+16 > len(body) {
			return nil, fmt.Errorf("exhq_transaction record %d truncated at offset %d", i, pos)
		}
		raw := append([]byte(nil), body[pos:pos+16]...)
		timeRaw := binary.LittleEndian.Uint16(raw[0:2])
		nature := binary.LittleEndian.Uint16(raw[14:16])
		natureMark := int(nature) / 10000
		natureValue := int(nature) % 10000
		second := natureValue
		if second > 59 {
			second = 0
		}
		out = append(out, model.Transaction{
			Market:         market,
			Code:           code,
			Date:           date,
			Hour:           int(timeRaw / 60),
			Minute:         int(timeRaw % 60),
			Second:         second,
			Price:          float64(binary.LittleEndian.Uint32(raw[2:6])),
			Volume:         binary.LittleEndian.Uint32(raw[6:10]),
			PositionChange: int32(binary.LittleEndian.Uint32(raw[10:14])),
			Nature:         nature,
			NatureMark:     natureMark,
			NatureValue:    natureValue,
			Direction:      transactionDirection(market, nature),
			Unknown:        append([]byte(nil), unknown...),
			Raw:            raw,
		})
		pos += 16
	}
	return out, nil
}

func transactionDirection(market model.MarketID, nature uint16) int {
	if market == model.MarketID(31) || market == model.MarketID(48) {
		switch nature {
		case 0:
			return 1
		case 256:
			return -1
		default:
			return 0
		}
	}
	switch int(nature) / 10000 {
	case 0:
		return 1
	case 1:
		return -1
	default:
		return 0
	}
}
