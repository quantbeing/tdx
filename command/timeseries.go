package command

import (
	"encoding/binary"
	"fmt"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

type MinuteTimeDataCommand struct {
	Market  model.Market
	Code    string
	Date    int
	History bool
}

func NewMinuteTimeDataCommand(market model.Market, code string) MinuteTimeDataCommand {
	return MinuteTimeDataCommand{Market: market, Code: code}
}

func NewHistoryMinuteTimeDataCommand(market model.Market, code string, date int) MinuteTimeDataCommand {
	return MinuteTimeDataCommand{Market: market, Code: code, Date: date, History: true}
}

func (c MinuteTimeDataCommand) Operation() string {
	if c.History {
		return "history_minute_time"
	}
	return "minute_time"
}

func (c MinuteTimeDataCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	if c.History {
		req := mustHex("0c01300001010d000d00b40f")
		req = binary.LittleEndian.AppendUint32(req, uint32(c.Date))
		req = append(req, byte(c.Market))
		req = append(req, code[:]...)
		return req, nil
	}
	req := mustHex("0c1b080001010e000e001d05")
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint32(req, 0)
	return req, nil
}

func (c MinuteTimeDataCommand) ParseResponse(body []byte) (any, error) {
	skip := 4
	if c.History {
		skip = 6
	}
	return parseMinuteBody(body, skip)
}

func parseMinuteBody(body []byte, skip int) ([]model.MinuteTime, error) {
	if len(body) < skip {
		return nil, fmt.Errorf("minute_time response truncated: %d < skip %d", len(body), skip)
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := skip
	lastPrice := 0
	out := make([]model.MinuteTime, 0, count)
	for i := 0; i < count; i++ {
		start := pos
		priceDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("minute_time price[%d]: %w", i, err)
		}
		pos = next
		unknown1, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("minute_time unknown1[%d]: %w", i, err)
		}
		pos = next
		vol, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("minute_time volume[%d]: %w", i, err)
		}
		pos = next
		lastPrice += priceDiff
		out = append(out, model.MinuteTime{
			Price:    model.NewDecimal(int64(lastPrice), 2),
			Volume:   float64(vol),
			Unknown1: unknown1,
			Raw:      append([]byte(nil), body[start:pos]...),
		})
	}
	return out, nil
}

type TransactionDataCommand struct {
	Market  model.Market
	Code    string
	Date    int
	Start   int
	Count   int
	History bool
}

func NewTransactionDataCommand(market model.Market, code string, start int, count int) TransactionDataCommand {
	return TransactionDataCommand{Market: market, Code: code, Start: start, Count: count}
}

func NewHistoryTransactionDataCommand(market model.Market, code string, date int, start int, count int) TransactionDataCommand {
	return TransactionDataCommand{Market: market, Code: code, Date: date, Start: start, Count: count, History: true}
}

func (c TransactionDataCommand) Operation() string {
	if c.History {
		return "history_transaction"
	}
	return "transaction"
}

func (c TransactionDataCommand) BuildRequest() ([]byte, error) {
	code := [6]byte{}
	copy(code[:], []byte(c.Code))
	if c.History {
		req := mustHex("0c013001000112001200b50f")
		req = binary.LittleEndian.AppendUint32(req, uint32(c.Date))
		req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
		req = append(req, code[:]...)
		req = binary.LittleEndian.AppendUint16(req, uint16(c.Start))
		req = binary.LittleEndian.AppendUint16(req, uint16(c.Count))
		return req, nil
	}
	req := mustHex("0c17080101010e000e00c50f")
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Market))
	req = append(req, code[:]...)
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Start))
	req = binary.LittleEndian.AppendUint16(req, uint16(c.Count))
	return req, nil
}

func (c TransactionDataCommand) ParseResponse(body []byte) (any, error) {
	if c.History {
		return parseTransactionBody(body, true)
	}
	return parseTransactionBody(body, false)
}

func parseTransactionBody(body []byte, history bool) ([]model.Transaction, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("transaction response truncated: %d", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	if history {
		if len(body) < 6 {
			return nil, fmt.Errorf("history_transaction response truncated: %d", len(body))
		}
		pos = 6
	}
	lastPrice := 0
	out := make([]model.Transaction, 0, count)
	for i := 0; i < count; i++ {
		start := pos
		hour, minute, next, err := codec.GetTime(body, pos)
		if err != nil {
			return nil, fmt.Errorf("transaction time[%d]: %w", i, err)
		}
		pos = next
		priceDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("transaction price[%d]: %w", i, err)
		}
		pos = next
		vol, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("transaction vol[%d]: %w", i, err)
		}
		pos = next
		numOrders := 0
		if !history {
			numOrders, next, err = codec.GetPrice(body, pos)
			if err != nil {
				return nil, fmt.Errorf("transaction num_orders[%d]: %w", i, err)
			}
			pos = next
		}
		buyOrSell, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("transaction buyorsell[%d]: %w", i, err)
		}
		pos = next
		unknownLast, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("transaction unknown_last[%d]: %w", i, err)
		}
		pos = next
		lastPrice += priceDiff
		out = append(out, model.Transaction{
			Hour: hour, Minute: minute, Price: model.NewDecimal(int64(lastPrice), 2),
			Vol: vol, NumOrders: numOrders, BuyOrSell: buyOrSell, UnknownLast: unknownLast,
			Raw: append([]byte(nil), body[start:pos]...),
		})
	}
	return out, nil
}
