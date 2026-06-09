package tdx

import (
	"context"
	"testing"

	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

func BenchmarkClientGetSecurityQuotesBatchSplit(b *testing.B) {
	symbols := make([]model.Symbol, 160)
	for i := range symbols {
		symbols[i] = model.Symbol{Market: model.MarketSH, Code: "600519"}
	}
	client := NewClient(Options{
		Servers:     []model.Server{{Name: "bench", Host: "bench", Port: 7709}},
		MaxAttempts: 1,
		Pool:        PoolOptions{Disable: true},
		Dialer: DialerFunc(func(context.Context, model.Server, TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				quoteCmd := cmd.(command.SecurityQuotesCommand)
				quotes := make([]model.Quote, len(quoteCmd.Symbols))
				for i, symbol := range quoteCmd.Symbols {
					quotes[i] = model.Quote{
						Market: symbol.Market,
						Code:   symbol.Code,
						Price:  model.NewPriceFromMilli(10500),
						Raw:    []byte{1},
					}
				}
				return quotes, nil
			}), nil
		}),
	})
	defer client.Close()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		quotes, err := client.GetSecurityQuotes(ctx, symbols)
		if err != nil {
			b.Fatal(err)
		}
		if len(quotes) != len(symbols) {
			b.Fatalf("quotes = %d, want %d", len(quotes), len(symbols))
		}
	}
}
