package command

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/quantbeing/tdx/codec"
	"github.com/quantbeing/tdx/model"
)

const MaxQuoteBatch = 80

type SecurityQuotesCommand struct {
	Symbols []model.Symbol
}

func NewSecurityQuotesCommand(symbols []model.Symbol) SecurityQuotesCommand {
	return SecurityQuotesCommand{Symbols: append([]model.Symbol(nil), symbols...)}
}

func (c SecurityQuotesCommand) Operation() string { return "security_quotes" }

func (c SecurityQuotesCommand) BuildRequest() ([]byte, error) {
	if len(c.Symbols) == 0 {
		return nil, fmt.Errorf("security_quotes requires at least one symbol")
	}
	if len(c.Symbols) > MaxQuoteBatch {
		return nil, fmt.Errorf("security_quotes batch too large: %d > %d", len(c.Symbols), MaxQuoteBatch)
	}
	payloadLen := len(c.Symbols)*7 + 12
	req := make([]byte, 0, payloadLen+16)
	req = binary.LittleEndian.AppendUint16(req, 0x010c)
	req = binary.LittleEndian.AppendUint32(req, 0x02006320)
	req = binary.LittleEndian.AppendUint16(req, uint16(payloadLen))
	req = binary.LittleEndian.AppendUint16(req, uint16(payloadLen))
	req = binary.LittleEndian.AppendUint32(req, 0x0005053e)
	req = binary.LittleEndian.AppendUint32(req, 0)
	req = binary.LittleEndian.AppendUint16(req, 0)
	req = binary.LittleEndian.AppendUint16(req, uint16(len(c.Symbols)))
	for _, sym := range c.Symbols {
		req = append(req, byte(sym.Market))
		code := [6]byte{}
		copy(code[:], []byte(sym.Code))
		req = append(req, code[:]...)
	}
	return req, nil
}

func (c SecurityQuotesCommand) ParseResponse(body []byte) (any, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("security_quotes response truncated: %d", len(body))
	}
	pos := 2
	count := int(binary.LittleEndian.Uint16(body[pos : pos+2]))
	pos += 2
	out := make([]model.Quote, 0, count)
	for i := 0; i < count; i++ {
		start := pos
		if pos+9 > len(body) {
			return nil, fmt.Errorf("security_quotes record %d header truncated", i)
		}
		market := model.Market(body[pos])
		code := strings.TrimRight(string(body[pos+1:pos+7]), "\x00 ")
		active1 := binary.LittleEndian.Uint16(body[pos+7 : pos+9])
		pos += 9

		priceRaw, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes price[%d]: %w", i, err)
		}
		pos = next
		lastCloseDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes pre_close[%d]: %w", i, err)
		}
		pos = next
		openDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes open[%d]: %w", i, err)
		}
		pos = next
		highDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes high[%d]: %w", i, err)
		}
		pos = next
		lowDiff, next, err := codec.GetPrice(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes low[%d]: %w", i, err)
		}
		pos = next
		unknown0, pos, err := getQuotePrice(body, pos, i, "unknown0")
		if err != nil {
			return nil, err
		}
		unknown1, pos, err := getQuotePrice(body, pos, i, "unknown1")
		if err != nil {
			return nil, err
		}
		vol, pos, err := getQuotePrice(body, pos, i, "vol")
		if err != nil {
			return nil, err
		}
		curVol, pos, err := getQuotePrice(body, pos, i, "cur_vol")
		if err != nil {
			return nil, err
		}
		amount, next, err := codec.GetVolume(body, pos)
		if err != nil {
			return nil, fmt.Errorf("security_quotes amount[%d]: %w", i, err)
		}
		pos = next
		sVol, pos, err := getQuotePrice(body, pos, i, "s_vol")
		if err != nil {
			return nil, err
		}
		bVol, pos, err := getQuotePrice(body, pos, i, "b_vol")
		if err != nil {
			return nil, err
		}
		unknown2, pos, err := getQuotePrice(body, pos, i, "unknown2")
		if err != nil {
			return nil, err
		}
		unknown3, pos, err := getQuotePrice(body, pos, i, "unknown3")
		if err != nil {
			return nil, err
		}

		var bid, ask [5]model.QuoteLevel
		for level := 0; level < 5; level++ {
			bidDelta, n, err := codec.GetPrice(body, pos)
			if err != nil {
				return nil, fmt.Errorf("security_quotes bid%d[%d]: %w", level+1, i, err)
			}
			pos = n
			askDelta, n, err := codec.GetPrice(body, pos)
			if err != nil {
				return nil, fmt.Errorf("security_quotes ask%d[%d]: %w", level+1, i, err)
			}
			pos = n
			bidVol, n, err := codec.GetPrice(body, pos)
			if err != nil {
				return nil, fmt.Errorf("security_quotes bid_vol%d[%d]: %w", level+1, i, err)
			}
			pos = n
			askVol, n, err := codec.GetPrice(body, pos)
			if err != nil {
				return nil, fmt.Errorf("security_quotes ask_vol%d[%d]: %w", level+1, i, err)
			}
			pos = n
			bid[level] = model.QuoteLevel{Price: model.NewDecimal(int64(priceRaw+bidDelta), 2), Volume: float64(bidVol)}
			ask[level] = model.QuoteLevel{Price: model.NewDecimal(int64(priceRaw+askDelta), 2), Volume: float64(askVol)}
		}
		if pos+2 > len(body) {
			return nil, fmt.Errorf("security_quotes tail flag[%d] truncated", i)
		}
		unknown4 := binary.LittleEndian.Uint16(body[pos : pos+2])
		pos += 2
		unknown5, pos, err := getQuotePrice(body, pos, i, "unknown5")
		if err != nil {
			return nil, err
		}
		unknown6, pos, err := getQuotePrice(body, pos, i, "unknown6")
		if err != nil {
			return nil, err
		}
		unknown7, pos, err := getQuotePrice(body, pos, i, "unknown7")
		if err != nil {
			return nil, err
		}
		unknown8, pos, err := getQuotePrice(body, pos, i, "unknown8")
		if err != nil {
			return nil, err
		}
		if pos+4 > len(body) {
			return nil, fmt.Errorf("security_quotes tail[%d] truncated", i)
		}
		riseSpeed := int16(binary.LittleEndian.Uint16(body[pos : pos+2]))
		active2 := binary.LittleEndian.Uint16(body[pos+2 : pos+4])
		pos += 4

		out = append(out, model.Quote{
			Market: market, Code: code, Active1: active1, Active2: active2,
			Price: model.NewDecimal(int64(priceRaw), 2), PreClose: model.NewDecimal(int64(priceRaw+lastCloseDiff), 2),
			Open: model.NewDecimal(int64(priceRaw+openDiff), 2), High: model.NewDecimal(int64(priceRaw+highDiff), 2),
			Low: model.NewDecimal(int64(priceRaw+lowDiff), 2), Vol: float64(vol), CurVol: float64(curVol),
			Amount: amount, SVol: float64(sVol), BVol: float64(bVol), Bid: bid, Ask: ask,
			RiseSpeed: model.NewDecimal(int64(riseSpeed), 2), Unknown0: unknown0, Unknown1: unknown1,
			Unknown2: unknown2, Unknown3: unknown3, Unknown4: unknown4, Unknown5: unknown5,
			Unknown6: unknown6, Unknown7: unknown7, Unknown8: unknown8, ServerTime: FormatServerTime(unknown0),
			Raw: append([]byte(nil), body[start:pos]...),
		})
	}
	return out, nil
}

func getQuotePrice(body []byte, pos int, idx int, field string) (int, int, error) {
	v, next, err := codec.GetPrice(body, pos)
	if err != nil {
		return 0, pos, fmt.Errorf("security_quotes %s[%d]: %w", field, idx, err)
	}
	return v, next, nil
}

func FormatServerTime(raw int) string {
	hours := raw / 1_000_000
	fractionalHour := raw % 1_000_000
	totalMillis := fractionalHour * 3600 / 1000
	minutes := totalMillis / 60_000
	remainder := totalMillis % 60_000
	seconds := remainder / 1000
	millis := remainder % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}
