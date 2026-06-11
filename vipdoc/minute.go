package vipdoc

import (
	"context"
	"fmt"

	"github.com/quantbeing/tdx/model"
)

type MinutePeriod uint8

const (
	MinutePeriod1 MinutePeriod = 1
	MinutePeriod5 MinutePeriod = 5
)

func (p MinutePeriod) String() string {
	return fmt.Sprintf("%d", p)
}

type MinuteBar struct {
	Market model.Market
	Code   string
	Date   int
	Hour   int
	Minute int
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Amount float64
	Volume uint32
	Raw    []byte
}

func (r *Reader) Minute(ctx context.Context, symbol model.Symbol, period MinutePeriod) ([]MinuteBar, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	switch period {
	case MinutePeriod1, MinutePeriod5:
		path, err := r.minutePath(symbol, period)
		if err != nil {
			return nil, err
		}
		return nil, &UnsupportedError{
			Path:    path,
			Period:  period,
			Details: "TDX .lc1/.lc5 local minute record layout is not confirmed enough for this package",
			Err:     ErrUnsupportedFormat,
		}
	default:
		return nil, &UnsupportedError{
			Period:  period,
			Details: "only 1-minute and 5-minute local files are considered for future support",
			Err:     ErrUnsupportedPeriod,
		}
	}
}
