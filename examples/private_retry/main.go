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
	servers := []model.Server{
		{Name: "private-a", Host: "10.0.0.10", Port: 7709},
		{Name: "private-b", Host: "10.0.0.11", Port: 7709},
	}

	client := tdx.NewClient(tdx.Options{
		Servers:     servers,
		Timeout:     2 * time.Second,
		MaxAttempts: 4,
		Transport: tdx.TransportOptions{
			ConnectTimeout: 500 * time.Millisecond,
			WriteTimeout:   500 * time.Millisecond,
			ReadTimeout:    2 * time.Second,
		},
		Retry: tdx.RetryOptions{
			Strategy:         tdx.RetryStrategySameHostFirst,
			SameHostAttempts: 2,
		},
		Pool: tdx.PoolOptions{MaxIdlePerHost: 2},
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Override one request chain without changing client defaults.
	ctx = tdx.WithRequestOptions(ctx, tdx.RequestOptions{
		MaxAttempts: 2,
		Retry: tdx.RetryOptions{
			Strategy: tdx.RetryStrategyFailoverFirst,
		},
		TimeoutPolicy: tdx.TimeoutPolicy{
			OperationTimeouts: map[string]time.Duration{
				"security_quotes": 800 * time.Millisecond,
			},
		},
	})

	quotes, err := client.GetSecurityQuotes(ctx, []model.Symbol{{Market: model.MarketSH, Code: "600519"}})
	if err != nil {
		log.Fatalf("quotes: %v", err)
	}
	fmt.Printf("%s %s %s\n", quotes[0].Market, quotes[0].Code, quotes[0].Price.String())
}
