package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/quantbeing/tdx/model"
)

type LiveClient interface {
	GetSecurityCount(context.Context, model.Market) (uint16, error)
	GetSecurityList(context.Context, model.Market, int) ([]model.Security, error)
	GetBars(context.Context, model.Market, string, model.KlineCategory, int, int) ([]model.Bar, error)
	GetSecurityQuotes(context.Context, []model.Symbol) ([]model.Quote, error)
	GetMinuteTimeData(context.Context, model.Market, string) ([]model.MinuteTime, error)
	GetTransactionData(context.Context, model.Market, string, int, int) ([]model.Transaction, error)
	GetMarketStat(context.Context) (model.MarketStat, error)
	GetFundFlow(context.Context, model.Market, string) (model.FundFlow, error)
	GetHistoryFundFlow(context.Context, model.Market, string, int, int) ([]model.HistoricalFundFlow, error)
	GetFinanceInfo(context.Context, model.Market, string) (model.FinanceInfo, error)
	GetXdxrInfo(context.Context, model.Market, string) ([]model.XdxrRecord, error)
	GetCompanyInfoCategory(context.Context, model.Market, string) ([]model.CompanyInfoCategory, error)
	ListBoards(context.Context, string) ([]model.Board, error)
	GetReportFile(context.Context, string) ([]byte, error)
}

type LiveOptions struct {
	Markets              []model.Market
	Symbols              []model.Symbol
	KlineCategories      []model.KlineCategory
	BarCount             int
	TransactionCount     int
	HistoryFundFlowCount int
	PerOperationTimeout  time.Duration
	BoardTypes           []string
	ReportFiles          []string
	SkipBoards           bool
	SkipReportFiles      bool
}

func DefaultLiveOptions() LiveOptions {
	return LiveOptions{
		Markets:              []model.Market{model.MarketSH, model.MarketSZ, model.MarketBJ},
		Symbols:              []model.Symbol{{Market: model.MarketSH, Code: "600519"}, {Market: model.MarketSZ, Code: "000001"}},
		KlineCategories:      []model.KlineCategory{model.KlineDay},
		BarCount:             10,
		TransactionCount:     50,
		HistoryFundFlowCount: 10,
		PerOperationTimeout:  8 * time.Second,
		BoardTypes:           []string{"concept"},
		ReportFiles:          []string{"base_info.zip"},
	}
}

func RunLive(ctx context.Context, client LiveClient, opts LiveOptions) Report {
	opts = normalizeLiveOptions(opts)
	report := NewReport()
	counts := make(map[model.Market]uint16, len(opts.Markets))
	for _, market := range opts.Markets {
		operation := fmt.Sprintf("security_count_%s", market.String())
		result, count := timedValue(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) (CheckResult, uint16) {
			count, err := client.GetSecurityCount(opCtx, market)
			if err != nil {
				return errorResult(operation, market, "", err), 0
			}
			return ValidateSecurityCount(operation, market, count), count
		})
		if result.OK {
			counts[market] = count
		}
		report.Add(result)

		operation = fmt.Sprintf("security_list_%s_0", market.String())
		result, securities := timedValue(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) (CheckResult, []model.Security) {
			items, err := client.GetSecurityList(opCtx, market, 0)
			if err != nil {
				return errorResult(operation, market, "", err), nil
			}
			return ValidateSecurities(operation, market, items), items
		})
		if count, ok := counts[market]; ok && len(securities) > int(count) {
			result.Findings = append(result.Findings, Finding{
				Severity:  SeverityError,
				Operation: operation,
				Market:    market,
				Field:     "rows",
				Message:   fmt.Sprintf("security list page rows = %d, market count = %d", len(securities), count),
			})
			result.OK = false
		}
		report.Add(result)
	}

	if len(opts.Symbols) > 0 {
		operation := "security_quotes"
		result := timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			quotes, err := client.GetSecurityQuotes(opCtx, opts.Symbols)
			if err != nil {
				return errorResult(operation, 0, "", err)
			}
			return ValidateQuotes(operation, opts.Symbols, quotes)
		})
		report.Add(result)

		symbol := opts.Symbols[0]
		for _, category := range opts.KlineCategories {
			operation := fmt.Sprintf("bars_%s_%s_%d", symbol.Market.String(), symbol.Code, category)
			result := timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
				bars, err := client.GetBars(opCtx, symbol.Market, symbol.Code, category, 0, opts.BarCount)
				if err != nil {
					return errorResult(operation, symbol.Market, symbol.Code, err)
				}
				return ValidateBars(operation, bars, opts.BarCount)
			})
			report.Add(result)
		}

		operation = fmt.Sprintf("minute_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			rows, err := client.GetMinuteTimeData(opCtx, symbol.Market, symbol.Code)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateMinuteTimes(operation, rows)
		})
		report.Add(result)

		operation = fmt.Sprintf("transaction_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			rows, err := client.GetTransactionData(opCtx, symbol.Market, symbol.Code, 0, opts.TransactionCount)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateTransactions(operation, rows)
		})
		report.Add(result)

		operation = "market_stat"
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			stat, err := client.GetMarketStat(opCtx)
			if err != nil {
				return errorResult(operation, 0, "", err)
			}
			return ValidateMarketStat(operation, stat)
		})
		report.Add(result)

		operation = fmt.Sprintf("fund_flow_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			flow, err := client.GetFundFlow(opCtx, symbol.Market, symbol.Code)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateFundFlow(operation, symbol.Market, symbol.Code, flow)
		})
		report.Add(result)

		operation = fmt.Sprintf("history_fund_flow_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			rows, err := client.GetHistoryFundFlow(opCtx, symbol.Market, symbol.Code, 0, opts.HistoryFundFlowCount)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateHistoricalFundFlow(operation, symbol.Market, symbol.Code, rows)
		})
		report.Add(result)

		operation = fmt.Sprintf("finance_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			info, err := client.GetFinanceInfo(opCtx, symbol.Market, symbol.Code)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateFinanceInfo(operation, info)
		})
		report.Add(result)

		operation = fmt.Sprintf("xdxr_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			rows, err := client.GetXdxrInfo(opCtx, symbol.Market, symbol.Code)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateXdxr(operation, rows)
		})
		report.Add(result)

		operation = fmt.Sprintf("company_%s_%s", symbol.Market.String(), symbol.Code)
		result = timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
			rows, err := client.GetCompanyInfoCategory(opCtx, symbol.Market, symbol.Code)
			if err != nil {
				return errorResult(operation, symbol.Market, symbol.Code, err)
			}
			return ValidateCompanyInfoCategories(operation, symbol.Market, symbol.Code, rows)
		})
		report.Add(result)
	}

	if !opts.SkipBoards {
		for _, boardType := range opts.BoardTypes {
			operation := "boards_" + boardType
			result := timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
				boards, err := client.ListBoards(opCtx, boardType)
				if err != nil {
					return errorResult(operation, 0, boardType, err)
				}
				return ValidateBoards(operation, boards)
			})
			report.Add(result)
		}
	}

	if !opts.SkipReportFiles {
		for _, filename := range opts.ReportFiles {
			operation := "report_file_" + filename
			result := timedCheck(ctx, opts.PerOperationTimeout, operation, func(opCtx context.Context) CheckResult {
				data, err := client.GetReportFile(opCtx, filename)
				if err != nil {
					return errorResult(operation, 0, filename, err)
				}
				return ValidateBytes(operation, data, 1)
			})
			report.Add(result)
		}
	}

	report.Finish()
	return report
}

func normalizeLiveOptions(opts LiveOptions) LiveOptions {
	defaults := DefaultLiveOptions()
	if len(opts.Markets) == 0 {
		opts.Markets = defaults.Markets
	}
	if len(opts.Symbols) == 0 {
		opts.Symbols = defaults.Symbols
	}
	if len(opts.KlineCategories) == 0 {
		opts.KlineCategories = defaults.KlineCategories
	}
	if opts.BarCount <= 0 {
		opts.BarCount = defaults.BarCount
	}
	if opts.TransactionCount <= 0 {
		opts.TransactionCount = defaults.TransactionCount
	}
	if opts.HistoryFundFlowCount <= 0 {
		opts.HistoryFundFlowCount = defaults.HistoryFundFlowCount
	}
	if opts.PerOperationTimeout <= 0 {
		opts.PerOperationTimeout = defaults.PerOperationTimeout
	}
	if len(opts.BoardTypes) == 0 && !opts.SkipBoards {
		opts.BoardTypes = defaults.BoardTypes
	}
	if len(opts.ReportFiles) == 0 && !opts.SkipReportFiles {
		opts.ReportFiles = defaults.ReportFiles
	}
	return opts
}

func errorResult(operation string, market model.Market, code string, err error) CheckResult {
	result := CheckResult{Operation: operation, OK: false}
	result.add(SeverityError, market, code, 0, "error", err.Error())
	return result
}

func timedCheck(ctx context.Context, timeout time.Duration, operation string, fn func(context.Context) CheckResult) CheckResult {
	start := time.Now()
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result := fn(opCtx)
	result.Operation = operation
	result.LatencyMS = time.Since(start).Milliseconds()
	return result
}

func timedValue[T any](ctx context.Context, timeout time.Duration, operation string, fn func(context.Context) (CheckResult, T)) (CheckResult, T) {
	start := time.Now()
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, value := fn(opCtx)
	result.Operation = operation
	result.LatencyMS = time.Since(start).Milliseconds()
	return result, value
}
