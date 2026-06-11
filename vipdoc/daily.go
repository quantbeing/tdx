package vipdoc

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"github.com/quantbeing/tdx/model"
)

const dailyRecordSize = 32

type DailyBar struct {
	Market   model.Market
	Code     string
	Date     int
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Amount   float64
	Volume   uint32
	Reserved uint32

	RawOpen  uint32
	RawHigh  uint32
	RawLow   uint32
	RawClose uint32
	Raw      []byte
}

func (r *Reader) Daily(ctx context.Context, symbol model.Symbol) ([]DailyBar, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	path, err := r.dailyPath(symbol)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scale := priceScale(symbol)
	var offset int64
	var out []DailyBar
	var buf [dailyRecordSize]byte
	for {
		if err := contextErr(ctx); err != nil {
			return nil, err
		}
		n, err := io.ReadFull(file, buf[:])
		if errors.Is(err, io.EOF) && n == 0 {
			break
		}
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				return nil, newParseError(path, offset, dailyRecordSize, n, ErrTruncatedFile)
			}
			return nil, err
		}

		raw := append([]byte(nil), buf[:]...)
		rawOpen := binary.LittleEndian.Uint32(raw[4:8])
		rawHigh := binary.LittleEndian.Uint32(raw[8:12])
		rawLow := binary.LittleEndian.Uint32(raw[12:16])
		rawClose := binary.LittleEndian.Uint32(raw[16:20])
		out = append(out, DailyBar{
			Market:   symbol.Market,
			Code:     symbol.Code,
			Date:     int(binary.LittleEndian.Uint32(raw[0:4])),
			Open:     float64(rawOpen) * scale,
			High:     float64(rawHigh) * scale,
			Low:      float64(rawLow) * scale,
			Close:    float64(rawClose) * scale,
			Amount:   float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[20:24]))),
			Volume:   binary.LittleEndian.Uint32(raw[24:28]),
			Reserved: binary.LittleEndian.Uint32(raw[28:32]),
			RawOpen:  rawOpen,
			RawHigh:  rawHigh,
			RawLow:   rawLow,
			RawClose: rawClose,
			Raw:      raw,
		})
		offset += dailyRecordSize
	}
	return out, nil
}

func priceScale(symbol model.Symbol) float64 {
	switch symbol.Market {
	case model.MarketSH, model.MarketSZ:
		return 0.01
	default:
		return 0.01
	}
}
