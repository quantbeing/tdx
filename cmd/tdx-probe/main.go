package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/model"
)

func main() {
	var op string
	var market string
	var timeout time.Duration
	var captureDir string
	flag.StringVar(&op, "op", "security-count", "operation: security-count, security-list, stock-bars, index-bars, quote, market-stat, minute, transaction, fund-flow, history-fund-flow, finance, xdxr, company, block-meta, block, report")
	flag.StringVar(&market, "market", "sh", "market: sh, sz, bj")
	flag.DurationVar(&timeout, "timeout", 8*time.Second, "timeout")
	flag.StringVar(&captureDir, "capture-dir", "", "write raw response fixture JSON to this directory")
	flag.Parse()

	client := tdx.NewClient(tdx.Options{Timeout: timeout})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := commandFor(op, parseMarket(market))
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

func commandFor(op string, market model.Market) command.Command {
	switch strings.ToLower(op) {
	case "security-list":
		return command.NewSecurityListCommand(market, 0)
	case "stock-bars":
		return command.NewSecurityBarsCommand(model.MarketSH, "600519", model.KlineDay, 0, 10)
	case "index-bars":
		return command.NewIndexBarsCommand(model.MarketSH, "000001", model.KlineDay, 0, 10)
	case "quote":
		return command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "600519"}})
	case "market-stat":
		return command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "880005"}})
	case "minute":
		return command.NewMinuteTimeDataCommand(model.MarketSH, "600519")
	case "transaction":
		return command.NewTransactionDataCommand(model.MarketSH, "600519", 0, 50)
	case "fund-flow":
		return command.NewTransactionDataCommand(model.MarketSH, "600519", 0, 2000)
	case "history-fund-flow":
		return command.NewHistoryFundFlowCommand(model.MarketSH, "600519", 0, 10)
	case "finance":
		return command.NewFinanceInfoCommand(model.MarketSH, "600519")
	case "xdxr":
		return command.NewXdxrInfoCommand(model.MarketSH, "600519")
	case "company":
		return command.NewCompanyInfoCategoryCommand(model.MarketSH, "600519")
	case "block-meta":
		return command.NewBlockInfoMetaCommand("block_gn.dat")
	case "block":
		return command.NewBlockInfoCommand("block_gn.dat", 0, 30000)
	case "report":
		return command.NewReportFileCommand("base_info.zip", 0, 30000)
	default:
		return command.NewSecurityCountCommand(market)
	}
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
