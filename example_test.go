package tdx_test

import (
	"context"
	"fmt"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

type exampleRoundTripper struct{}

func (exampleRoundTripper) RoundTrip(_ context.Context, cmd command.Command) (any, error) {
	switch c := cmd.(type) {
	case command.SecurityCountCommand:
		switch c.Market {
		case model.MarketSH, model.MarketSZ:
			return uint16(1), nil
		default:
			return uint16(0), nil
		}
	case command.SecurityListCommand:
		switch c.Market {
		case model.MarketSH:
			return []model.Security{{Market: model.MarketSH, Code: "600519", Name: "Kweichow Moutai", DecimalPoint: 2}}, nil
		case model.MarketSZ:
			return []model.Security{{Market: model.MarketSZ, Code: "000001", Name: "Ping An Bank", DecimalPoint: 2}}, nil
		default:
			return []model.Security{}, nil
		}
	case command.SecurityQuotesCommand:
		out := make([]model.Quote, 0, len(c.Symbols))
		for _, sym := range c.Symbols {
			out = append(out, model.Quote{
				Market: sym.Market,
				Code:   sym.Code,
				Price:  model.NewDecimal(168888, 2),
				Bid: [5]model.QuoteLevel{
					{Price: model.NewDecimal(168887, 2), Volume: 1200},
				},
				Ask: [5]model.QuoteLevel{
					{Price: model.NewDecimal(168889, 2), Volume: 800},
				},
			})
		}
		return out, nil
	case command.BarsCommand:
		return []model.Bar{{
			Market:   c.Market,
			Code:     c.Code,
			Category: c.Category,
			Year:     2026,
			Month:    6,
			Day:      11,
			Open:     model.NewDecimal(168000, 2),
			High:     model.NewDecimal(169000, 2),
			Low:      model.NewDecimal(167500, 2),
			Close:    model.NewDecimal(168888, 2),
			Vol:      10000,
			Amount:   16888800,
		}}, nil
	default:
		return nil, fmt.Errorf("example round tripper does not handle %s", cmd.Operation())
	}
}

func (exampleRoundTripper) Close() error { return nil }

func exampleClient() *tdx.Client {
	return tdx.NewClient(tdx.Options{
		Servers:     []model.Server{{Name: "example", Host: "127.0.0.1", Port: 7709}},
		MaxAttempts: 1,
		Dialer: tdx.DialerFunc(func(context.Context, model.Server, tdx.TransportOptions) (tdx.RoundTripper, error) {
			return exampleRoundTripper{}, nil
		}),
	})
}

func ExampleNewClient() {
	client := tdx.NewClient(tdx.Options{
		Timeout:       2 * time.Second,
		MaxAttempts:   2,
		TimeoutPolicy: tdx.FastTimeoutPolicy(),
		Retry: tdx.RetryOptions{
			Strategy: tdx.RetryStrategyFailoverFirst,
		},
	})
	defer client.Close()

	fmt.Println("configured")
	// Output: configured
}

func ExampleClient_ListAShares() {
	client := exampleClient()
	defer client.Close()

	result, err := client.ListAShares(context.Background())
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(len(result.Items), result.Items[0].Code, result.Items[1].Code)
	// Output: 2 600519 000001
}

func ExampleClient_GetSecurityQuotes() {
	client := exampleClient()
	defer client.Close()

	quotes, err := client.GetSecurityQuotes(context.Background(), []model.Symbol{
		{Market: model.MarketSH, Code: "600519"},
		{Market: model.MarketSZ, Code: "000001"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(len(quotes), quotes[0].Code, quotes[0].Price.String(), quotes[0].Bid[0].Price.String())
	// Output: 2 600519 1688.88 1688.87
}

func ExampleClient_GetBars() {
	client := exampleClient()
	defer client.Close()

	bars, err := client.GetBars(context.Background(), model.MarketSH, "600519", model.KlineDay, 0, 1)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(bars[0].Code, bars[0].Year, bars[0].Close.String())
	// Output: 600519 2026 1688.88
}

func ExampleWithRequestOptions() {
	client := exampleClient()
	defer client.Close()

	ctx := tdx.WithRequestOptions(context.Background(), tdx.RequestOptions{
		MaxAttempts: 1,
		TimeoutPolicy: tdx.TimeoutPolicy{
			OperationTimeouts: map[string]time.Duration{
				"security_quotes": 500 * time.Millisecond,
			},
		},
	})

	quotes, err := client.GetSecurityQuotes(ctx, []model.Symbol{{Market: model.MarketSH, Code: "600519"}})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(len(quotes), quotes[0].Code)
	// Output: 1 600519
}
