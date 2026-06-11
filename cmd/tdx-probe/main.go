package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/internal/cmdflags"
	"github.com/quantbeing/tdx/model"
)

func main() {
	var op string
	var market string
	var code string
	var symbols string
	var date int
	var start int
	var count int
	var file string
	var timeout time.Duration
	var captureDir string
	var maxAttempts int
	var retryStrategyRaw string
	var sameHostAttempts int
	var rawHex string
	flag.StringVar(&op, "op", "security-count", "operation: security-count, security-list, stock-bars, index-bars, quote, market-stat, minute, history-minute, transaction, history-transaction, fund-flow, history-fund-flow, finance, xdxr, company, block-meta, block, report")
	flag.StringVar(&rawHex, "raw-hex", "", "diagnostic-only raw request hex; bypasses known operation builders")
	flag.StringVar(&market, "market", "sh", "market: sh, sz, bj")
	flag.StringVar(&code, "code", "", "security code for symbol operations")
	flag.StringVar(&symbols, "symbols", "", "comma-separated quote symbols as market:code, for example sh:600519,sz:000001")
	flag.IntVar(&date, "date", 0, "date for history operations, YYYYMMDD")
	flag.IntVar(&start, "start", 0, "start offset for paged operations")
	flag.IntVar(&count, "count", 0, "row count for paged operations")
	flag.StringVar(&file, "file", "", "filename for block/report operations")
	flag.DurationVar(&timeout, "timeout", 8*time.Second, "timeout")
	flag.StringVar(&captureDir, "capture-dir", "", "write raw response fixture JSON to this directory")
	flag.IntVar(&maxAttempts, "max-attempts", 0, "maximum attempts per request; 0 uses client default")
	flag.StringVar(&retryStrategyRaw, "retry-strategy", "failover-first", "retry strategy: failover-first or same-host-first")
	flag.IntVar(&sameHostAttempts, "same-host-attempts", 1, "same-host attempts for same-host-first retry strategy")
	flag.Parse()

	retry, err := cmdflags.RetryOptions(retryStrategyRaw, sameHostAttempts)
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"operation": op,
			"ok":        false,
			"error":     err.Error(),
		})
		os.Exit(2)
	}
	client := tdx.NewClient(tdx.Options{
		Timeout:     timeout,
		MaxAttempts: maxAttempts,
		Retry:       retry,
	})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd, err := commandForOptions(probeOptions{
		Op: op, Market: market, Code: code, Symbols: symbols,
		Date: date, Start: start, Count: count, File: file, RawHex: rawHex,
	})
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"operation": op,
			"ok":        false,
			"error":     err.Error(),
		})
		os.Exit(2)
	}
	if captureDir != "" {
		capture, err := client.Capture(ctx, cmd)
		if err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"operation": cmd.Operation(),
				"ok":        false,
				"error":     err.Error(),
			})
			os.Exit(1)
		}
		summary, err := writeCaptureFixture(captureDir, capture)
		if err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"operation": cmd.Operation(),
				"ok":        false,
				"error":     err.Error(),
			})
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(summary)
		return
	}
	result := client.HealthCheck(ctx, cmd)
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func writeCaptureFixture(dir string, capture tdx.CapturedResponse) (diagnostic.FixtureSummary, error) {
	return diagnostic.WriteFixture(diagnostic.DefaultFixturePath(dir, capture), capture)
}

type probeOptions struct {
	Op      string
	Market  string
	Code    string
	Symbols string
	Date    int
	Start   int
	Count   int
	File    string
	RawHex  string
}

func commandForOptions(opts probeOptions) (command.Command, error) {
	if strings.TrimSpace(opts.RawHex) != "" {
		request, err := parseRawHex(opts.RawHex)
		if err != nil {
			return nil, err
		}
		return rawProbeCommand{Request: request}, nil
	}
	market := parseMarket(opts.Market)
	code := defaultCode(opts.Code, market)
	count := opts.Count
	if count <= 0 {
		count = 10
	}
	switch strings.ToLower(opts.Op) {
	case "security-list":
		return command.NewSecurityListCommand(market, opts.Start), nil
	case "stock-bars":
		return command.NewSecurityBarsCommand(market, code, model.KlineDay, opts.Start, count), nil
	case "index-bars":
		if opts.Code == "" {
			code = "000001"
			market = model.MarketSH
		}
		return command.NewIndexBarsCommand(market, code, model.KlineDay, opts.Start, count), nil
	case "quote":
		symbols, err := parseSymbols(opts.Symbols)
		if err != nil {
			return nil, err
		}
		if len(symbols) == 0 {
			symbols = []model.Symbol{{Market: market, Code: code}}
		}
		return command.NewSecurityQuotesCommand(symbols), nil
	case "market-stat":
		return command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "880005"}}), nil
	case "minute":
		return command.NewMinuteTimeDataCommand(market, code), nil
	case "history-minute":
		if opts.Date <= 0 {
			return nil, fmt.Errorf("history-minute requires -date YYYYMMDD")
		}
		return command.NewHistoryMinuteTimeDataCommand(market, code, opts.Date), nil
	case "transaction":
		if opts.Count <= 0 {
			count = 50
		}
		return command.NewTransactionDataCommand(market, code, opts.Start, count), nil
	case "history-transaction":
		if opts.Date <= 0 {
			return nil, fmt.Errorf("history-transaction requires -date YYYYMMDD")
		}
		if opts.Count <= 0 {
			count = 50
		}
		return command.NewHistoryTransactionDataCommand(market, code, opts.Date, opts.Start, count), nil
	case "fund-flow":
		if opts.Count <= 0 {
			count = 2000
		}
		return command.NewTransactionDataCommand(market, code, opts.Start, count), nil
	case "history-fund-flow":
		return command.NewHistoryFundFlowCommand(market, code, opts.Start, count), nil
	case "finance":
		return command.NewFinanceInfoCommand(market, code), nil
	case "xdxr":
		return command.NewXdxrInfoCommand(market, code), nil
	case "company":
		return command.NewCompanyInfoCategoryCommand(market, code), nil
	case "block-meta":
		return command.NewBlockInfoMetaCommand(defaultFile(opts.File, "block_gn.dat")), nil
	case "block":
		return command.NewBlockInfoCommand(defaultFile(opts.File, "block_gn.dat"), opts.Start, defaultCount(opts.Count, 30000)), nil
	case "report":
		return command.NewReportFileCommand(defaultFile(opts.File, "base_info.zip"), opts.Start, defaultCount(opts.Count, 30000)), nil
	default:
		return command.NewSecurityCountCommand(market), nil
	}
}

type rawProbeCommand struct {
	Request []byte
}

func (c rawProbeCommand) BuildRequest() ([]byte, error) {
	out := make([]byte, len(c.Request))
	copy(out, c.Request)
	return out, nil
}

func (c rawProbeCommand) ParseResponse(body []byte) (any, error) {
	out := make([]byte, len(body))
	copy(out, body)
	return out, nil
}

func (c rawProbeCommand) Operation() string {
	return "raw_probe"
}

func parseRawHex(raw string) ([]byte, error) {
	normalized := strings.NewReplacer(
		"0x", "",
		"0X", "",
		" ", "",
		"\n", "",
		"\t", "",
		",", "",
		"_", "",
	).Replace(strings.TrimSpace(raw))
	if normalized == "" {
		return nil, fmt.Errorf("raw-hex cannot be empty")
	}
	if len(normalized)%2 != 0 {
		return nil, fmt.Errorf("raw-hex must contain an even number of hex digits")
	}
	out, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, fmt.Errorf("raw-hex decode: %w", err)
	}
	return out, nil
}

func commandFor(op string, market model.Market) command.Command {
	cmd, err := commandForOptions(probeOptions{Op: op, Market: market.String()})
	if err != nil {
		return command.UnsupportedCommand{Name: op}
	}
	return cmd
}

func parseMarket(raw string) model.Market {
	switch strings.ToLower(raw) {
	case "sz":
		return model.MarketSZ
	case "bj":
		return model.MarketBJ
	default:
		return model.MarketSH
	}
}

func parseSymbols(raw string) ([]model.Symbol, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]model.Symbol, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.Split(part, ":")
		switch len(pair) {
		case 2:
			code := strings.TrimSpace(pair[1])
			if code == "" {
				return nil, fmt.Errorf("symbol %q has empty code", part)
			}
			out = append(out, model.Symbol{Market: parseMarket(pair[0]), Code: code})
		default:
			return nil, fmt.Errorf("symbol %q must use market:code", part)
		}
	}
	return out, nil
}

func defaultCode(code string, market model.Market) string {
	code = strings.TrimSpace(code)
	if code != "" {
		return code
	}
	switch market {
	case model.MarketSZ:
		return "000001"
	default:
		return "600519"
	}
}

func defaultFile(file string, fallback string) string {
	file = strings.TrimSpace(file)
	if file == "" {
		return fallback
	}
	return file
}

func defaultCount(count int, fallback int) int {
	if count <= 0 {
		return fallback
	}
	return count
}
