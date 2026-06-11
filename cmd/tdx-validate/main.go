package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/internal/cmdflags"
	"github.com/quantbeing/tdx/model"
	"github.com/quantbeing/tdx/validation"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, nil, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer, client validation.LiveClient, getenv func(string) string) error {
	var timeout time.Duration
	var operationTimeout time.Duration
	var connectTimeout time.Duration
	var allowLive bool
	var marketsRaw string
	var symbolsRaw string
	var klineRaw string
	var fullKline bool
	var barCount int
	var transactionCount int
	var historyFundFlowCount int
	var boardTypesRaw string
	var reportFilesRaw string
	var skipBoards bool
	var skipFiles bool
	var fullSecurityList bool
	var securityListPageRetries int
	var maxAttempts int
	var retryStrategyRaw string
	var sameHostAttempts int
	var pretty bool

	fs := flag.NewFlagSet("tdx-validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "overall validation timeout")
	fs.DurationVar(&operationTimeout, "operation-timeout", 8*time.Second, "timeout per validation operation")
	fs.DurationVar(&connectTimeout, "connect-timeout", 2*time.Second, "TCP connect/write timeout used by live client")
	fs.BoolVar(&allowLive, "allow-live", false, "run without TDX_LIVE=1")
	fs.StringVar(&marketsRaw, "markets", "sh,sz,bj", "comma-separated markets: sh,sz,bj")
	fs.StringVar(&symbolsRaw, "symbols", "sh:600519,sz:000001", "comma-separated symbols as market:code")
	fs.StringVar(&klineRaw, "kline", "day", "comma-separated kline categories")
	fs.BoolVar(&fullKline, "full-kline", false, "validate all known kline categories")
	fs.IntVar(&barCount, "bar-count", 10, "bar rows requested per kline category")
	fs.IntVar(&transactionCount, "transaction-count", 50, "transaction rows requested")
	fs.IntVar(&historyFundFlowCount, "history-fund-flow-count", 10, "historical fund flow rows requested")
	fs.StringVar(&boardTypesRaw, "board-types", "concept", "comma-separated board types")
	fs.StringVar(&reportFilesRaw, "report-files", "base_info.zip", "comma-separated report files")
	fs.BoolVar(&skipBoards, "skip-boards", false, "skip board validation")
	fs.BoolVar(&skipFiles, "skip-files", false, "skip report-file validation")
	fs.BoolVar(&fullSecurityList, "full-security-list", false, "validate every security-list page for each selected market")
	fs.IntVar(&securityListPageRetries, "security-list-page-retries", 0, "retry each full security-list page this many times after a page error")
	fs.IntVar(&maxAttempts, "max-attempts", 0, "maximum attempts per request; 0 uses client default")
	fs.StringVar(&retryStrategyRaw, "retry-strategy", "failover-first", "retry strategy: failover-first or same-host-first")
	fs.IntVar(&sameHostAttempts, "same-host-attempts", 1, "same-host attempts for same-host-first retry strategy")
	fs.BoolVar(&pretty, "pretty", false, "pretty-print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if getenv("TDX_LIVE") != "1" && !allowLive {
		return fmt.Errorf("refusing live TDX validation without TDX_LIVE=1")
	}
	markets, err := parseMarkets(marketsRaw)
	if err != nil {
		return err
	}
	symbols, err := parseSymbols(symbolsRaw)
	if err != nil {
		return err
	}
	categories, err := parseKlineCategories(klineRaw)
	if err != nil {
		return err
	}
	if fullKline {
		categories = allKlineCategories()
	}
	opts := validation.LiveOptions{
		Markets:                     markets,
		Symbols:                     symbols,
		KlineCategories:             categories,
		BarCount:                    barCount,
		TransactionCount:            transactionCount,
		HistoryFundFlowCount:        historyFundFlowCount,
		PerOperationTimeout:         operationTimeout,
		BoardTypes:                  splitCSV(boardTypesRaw),
		ReportFiles:                 splitCSV(reportFilesRaw),
		SkipBoards:                  skipBoards,
		SkipReportFiles:             skipFiles,
		FullSecurityList:            fullSecurityList,
		FullSecurityListPageRetries: securityListPageRetries,
	}

	ownedClient := false
	if client == nil {
		clientOpts, err := buildClientOptions(timeout, operationTimeout, connectTimeout, maxAttempts, retryStrategyRaw, sameHostAttempts)
		if err != nil {
			return err
		}
		client = tdx.NewClient(clientOpts)
		ownedClient = true
	}
	if ownedClient {
		if closer, ok := client.(interface{ Close() error }); ok {
			defer closer.Close()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	report := validation.RunLive(ctx, client, opts)
	enc := json.NewEncoder(stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(report)
}

func buildClientOptions(_ time.Duration, operationTimeout time.Duration, connectTimeout time.Duration, maxAttempts int, retryStrategyRaw string, sameHostAttempts int) (tdx.Options, error) {
	if operationTimeout <= 0 {
		operationTimeout = 8 * time.Second
	}
	if connectTimeout <= 0 || connectTimeout > operationTimeout {
		connectTimeout = operationTimeout
	}
	retry, err := cmdflags.RetryOptions(retryStrategyRaw, sameHostAttempts)
	if err != nil {
		return tdx.Options{}, err
	}
	return tdx.Options{
		MaxAttempts: maxAttempts,
		Timeout:     operationTimeout,
		Retry:       retry,
		Transport: tdx.TransportOptions{
			ConnectTimeout: connectTimeout,
			WriteTimeout:   connectTimeout,
			ReadTimeout:    operationTimeout,
		},
	}, nil
}

func parseMarkets(raw string) ([]model.Market, error) {
	parts := splitCSV(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("markets cannot be empty")
	}
	out := make([]model.Market, 0, len(parts))
	for _, part := range parts {
		market, err := parseMarket(part)
		if err != nil {
			return nil, err
		}
		out = append(out, market)
	}
	return out, nil
}

func parseSymbols(raw string) ([]model.Symbol, error) {
	parts := splitCSV(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("symbols cannot be empty")
	}
	out := make([]model.Symbol, 0, len(parts))
	for _, part := range parts {
		pair := strings.Split(part, ":")
		if len(pair) != 2 {
			return nil, fmt.Errorf("symbol %q must use market:code", part)
		}
		market, err := parseMarket(pair[0])
		if err != nil {
			return nil, err
		}
		code := strings.TrimSpace(pair[1])
		if code == "" {
			return nil, fmt.Errorf("symbol %q has empty code", part)
		}
		out = append(out, model.Symbol{Market: market, Code: code})
	}
	return out, nil
}

func parseKlineCategories(raw string) ([]model.KlineCategory, error) {
	parts := splitCSV(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("kline categories cannot be empty")
	}
	out := make([]model.KlineCategory, 0, len(parts))
	for _, part := range parts {
		category, err := parseKlineCategory(part)
		if err != nil {
			return nil, err
		}
		out = append(out, category)
	}
	return out, nil
}

func parseMarket(raw string) (model.Market, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sh", "1":
		return model.MarketSH, nil
	case "sz", "0":
		return model.MarketSZ, nil
	case "bj", "2":
		return model.MarketBJ, nil
	default:
		return 0, fmt.Errorf("unknown market %q", raw)
	}
}

func parseKlineCategory(raw string) (model.KlineCategory, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1m", "min1", "minute1":
		return model.KlineMinute1, nil
	case "3m", "min3", "minute3":
		return model.KlineMinute3, nil
	case "5m", "min5", "minute5":
		return model.KlineMinute5, nil
	case "15m", "min15", "minute15":
		return model.KlineMinute15, nil
	case "30m", "min30", "minute30":
		return model.KlineMinute30, nil
	case "60m", "min60", "minute60":
		return model.KlineMinute60, nil
	case "day", "d":
		return model.KlineDay, nil
	case "week", "w":
		return model.KlineWeek, nil
	case "month", "m":
		return model.KlineMonth, nil
	case "season", "quarter", "q":
		return model.KlineSeason, nil
	case "year", "y":
		return model.KlineYear, nil
	case "year-alt", "year_alt":
		return model.KlineYearAlt, nil
	default:
		return 0, fmt.Errorf("unknown kline category %q", raw)
	}
}

func allKlineCategories() []model.KlineCategory {
	return []model.KlineCategory{
		model.KlineMinute1,
		model.KlineMinute3,
		model.KlineMinute5,
		model.KlineMinute15,
		model.KlineMinute30,
		model.KlineMinute60,
		model.KlineDay,
		model.KlineWeek,
		model.KlineMonth,
		model.KlineSeason,
		model.KlineYear,
		model.KlineYearAlt,
	}
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
