package main

import (
	"context"
	"fmt"
	"log"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/model"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := tdx.NewClient(tdx.Options{
		Timeout:       2 * time.Second,
		MaxAttempts:   2,
		TimeoutPolicy: tdx.FastTimeoutPolicy(),
		Retry: tdx.RetryOptions{
			Strategy: tdx.RetryStrategyFailoverFirst,
		},
	})
	defer client.Close()

	ashares, err := client.ListASharesWithOptions(ctx, tdx.ListSecuritiesOptions{
		Markets:           []model.Market{model.MarketSH, model.MarketSZ},
		MaxPagesPerMarket: 1,
	})
	if err != nil && !tdx.IsPartialResultError(err) {
		log.Fatalf("list A shares: %v", err)
	}
	fmt.Printf("A-share sample rows: %d\n", len(ashares.Items))
	if len(ashares.Failures) > 0 {
		fmt.Printf("partial failures: %d\n", len(ashares.Failures))
	}

	quotes, err := client.GetSecurityQuotes(ctx, []model.Symbol{
		{Market: model.MarketSH, Code: "600519"},
		{Market: model.MarketSZ, Code: "000001"},
	})
	if err != nil {
		log.Fatalf("quotes: %v", err)
	}
	for _, quote := range quotes {
		fmt.Printf("%s %s last=%s bid1=%s ask1=%s\n",
			quote.Market,
			quote.Code,
			quote.Price.String(),
			quote.Bid[0].Price.String(),
			quote.Ask[0].Price.String(),
		)
	}

	bars, err := client.GetBars(ctx, model.MarketSH, "600519", model.KlineDay, 0, 5)
	if err != nil {
		log.Fatalf("bars: %v", err)
	}
	for _, bar := range bars {
		fmt.Printf("%04d-%02d-%02d close=%s volume=%.0f\n",
			bar.Year,
			bar.Month,
			bar.Day,
			bar.Close.String(),
			bar.Vol,
		)
	}
}
