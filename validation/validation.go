package validation

import (
	"fmt"
	"time"

	"github.com/quantbeing/tdx/model"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Finding struct {
	Severity  Severity     `json:"severity"`
	Operation string       `json:"operation"`
	Market    model.Market `json:"market,omitempty"`
	Code      string       `json:"code,omitempty"`
	Index     int          `json:"index,omitempty"`
	Field     string       `json:"field,omitempty"`
	Message   string       `json:"message"`
}

type CheckResult struct {
	Operation string    `json:"operation"`
	OK        bool      `json:"ok"`
	Rows      int       `json:"rows"`
	Findings  []Finding `json:"findings,omitempty"`
	LatencyMS int64     `json:"latency_ms,omitempty"`
}

type Summary struct {
	TotalResults  int `json:"total_results"`
	OKResults     int `json:"ok_results"`
	FailedResults int `json:"failed_results"`
	Errors        int `json:"errors"`
	Warnings      int `json:"warnings"`
	Rows          int `json:"rows"`
}

type Report struct {
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at,omitempty"`
	DurationMS int64         `json:"duration_ms"`
	Summary    Summary       `json:"summary"`
	Results    []CheckResult `json:"results"`
}

func NewReport() Report {
	return Report{StartedAt: time.Now()}
}

func (r *Report) Add(result CheckResult) {
	result.OK = result.OK && !hasError(result.Findings)
	r.Results = append(r.Results, result)
	r.Summary.TotalResults++
	if result.OK {
		r.Summary.OKResults++
	} else {
		r.Summary.FailedResults++
	}
	r.Summary.Rows += result.Rows
	for _, finding := range result.Findings {
		switch finding.Severity {
		case SeverityWarning:
			r.Summary.Warnings++
		default:
			r.Summary.Errors++
		}
	}
}

func (r *Report) Finish() {
	r.FinishedAt = time.Now()
	r.DurationMS = r.FinishedAt.Sub(r.StartedAt).Milliseconds()
}

func ValidateSecurities(operation string, market model.Market, items []model.Security) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(items)}
	if len(items) == 0 {
		result.add(SeverityError, market, "", 0, "rows", "empty security list")
		return result.finalize()
	}
	for i, item := range items {
		if item.Market != market {
			result.add(SeverityError, market, item.Code, i, "market", fmt.Sprintf("market = %s, want %s", item.Market, market))
		}
		if len(item.Code) != 6 {
			result.add(SeverityError, item.Market, item.Code, i, "code", "security code must be 6 characters")
		}
		if item.Name == "" {
			result.add(SeverityWarning, item.Market, item.Code, i, "name", "security name is empty")
		}
		if item.DecimalPoint > 6 {
			result.add(SeverityWarning, item.Market, item.Code, i, "decimal_point", fmt.Sprintf("decimal point %d is unusually high", item.DecimalPoint))
		}
		if len(item.Raw) != 29 {
			result.add(SeverityWarning, item.Market, item.Code, i, "raw", fmt.Sprintf("raw record size = %d, want 29", len(item.Raw)))
		}
	}
	return result.finalize()
}

func ValidateSecurityUniverse(operation string, market model.Market, expectedCount int, items []model.Security) CheckResult {
	result := ValidateSecurities(operation, market, items)
	if len(items) != expectedCount {
		result.add(SeverityError, market, "", 0, "rows", fmt.Sprintf("security list rows = %d, market count = %d", len(items), expectedCount))
	}
	seen := make(map[string]int, len(items))
	for i, item := range items {
		key := symbolKey(item.Market, item.Code)
		if first, ok := seen[key]; ok {
			result.add(SeverityError, item.Market, item.Code, i, "duplicate", fmt.Sprintf("duplicate security, first seen at index %d", first))
			continue
		}
		seen[key] = i
	}
	return result.finalize()
}

func ValidateSecurityCount(operation string, market model.Market, count uint16) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: int(count)}
	if count == 0 {
		result.add(SeverityError, market, "", 0, "count", "security count is zero")
	}
	return result.finalize()
}

func ValidateBars(operation string, bars []model.Bar, maxCount int) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(bars)}
	if len(bars) == 0 {
		result.add(SeverityError, 0, "", 0, "rows", "empty bar response")
		return result.finalize()
	}
	if maxCount > 0 && len(bars) > maxCount {
		result.add(SeverityError, 0, "", 0, "rows", fmt.Sprintf("bar rows = %d, max requested %d", len(bars), maxCount))
	}
	for i, bar := range bars {
		code := bar.Code
		if len(code) != 6 {
			result.add(SeverityWarning, bar.Market, code, i, "code", "bar code is not 6 characters")
		}
		if bar.Year <= 0 || bar.Month < 1 || bar.Month > 12 || bar.Day < 1 || bar.Day > 31 {
			result.add(SeverityError, bar.Market, code, i, "date", fmt.Sprintf("invalid date %04d-%02d-%02d", bar.Year, bar.Month, bar.Day))
		}
		if bar.Open.Mantissa < 0 || bar.Close.Mantissa < 0 || bar.High.Mantissa < 0 || bar.Low.Mantissa < 0 {
			result.add(SeverityError, bar.Market, code, i, "price", "bar price contains negative value")
		}
		if decimalLess(bar.High, bar.Open) || decimalLess(bar.High, bar.Close) || decimalLess(bar.High, bar.Low) {
			result.add(SeverityError, bar.Market, code, i, "high", "high is lower than another OHLC value")
		}
		if decimalLess(bar.Open, bar.Low) || decimalLess(bar.Close, bar.Low) || decimalLess(bar.High, bar.Low) {
			result.add(SeverityError, bar.Market, code, i, "low", "low is higher than another OHLC value")
		}
		if bar.Vol < 0 || bar.Amount < 0 {
			result.add(SeverityError, bar.Market, code, i, "volume_amount", "volume or amount is negative")
		}
		if len(bar.Raw) == 0 {
			result.add(SeverityWarning, bar.Market, code, i, "raw", "raw bar record is empty")
		}
	}
	return result.finalize()
}

func ValidateQuotes(operation string, requested []model.Symbol, quotes []model.Quote) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(quotes)}
	if len(quotes) != len(requested) {
		result.add(SeverityError, 0, "", 0, "rows", fmt.Sprintf("quote rows = %d, requested %d", len(quotes), len(requested)))
	}
	want := make(map[string]model.Symbol, len(requested))
	seen := make(map[string]bool, len(quotes))
	for _, symbol := range requested {
		want[symbolKey(symbol.Market, symbol.Code)] = symbol
	}
	for i, quote := range quotes {
		key := symbolKey(quote.Market, quote.Code)
		seen[key] = true
		if _, ok := want[key]; !ok {
			result.add(SeverityError, quote.Market, quote.Code, i, "symbol", "quote symbol was not requested")
		}
		if len(quote.Code) != 6 {
			result.add(SeverityError, quote.Market, quote.Code, i, "code", "quote code must be 6 characters")
		}
		if quote.Price.Mantissa < 0 {
			result.add(SeverityError, quote.Market, quote.Code, i, "price", "quote price is negative")
		}
		if quote.Vol < 0 || quote.Amount < 0 {
			result.add(SeverityError, quote.Market, quote.Code, i, "volume_amount", "volume or amount is negative")
		}
		if len(quote.Raw) == 0 {
			result.add(SeverityWarning, quote.Market, quote.Code, i, "raw", "raw quote record is empty")
		}
	}
	for _, symbol := range requested {
		if !seen[symbolKey(symbol.Market, symbol.Code)] {
			result.add(SeverityError, symbol.Market, symbol.Code, 0, "symbol", "requested quote symbol missing from response")
		}
	}
	return result.finalize()
}

func ValidateMinuteTimes(operation string, rows []model.MinuteTime) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(rows)}
	if len(rows) == 0 {
		result.add(SeverityError, 0, "", 0, "rows", "empty minute-time response")
		return result.finalize()
	}
	lastMinute := -1
	for i, row := range rows {
		minuteOfDay := row.Hour*60 + row.Minute
		if row.Hour < 0 || row.Hour > 23 || row.Minute < 0 || row.Minute > 59 {
			result.add(SeverityError, 0, "", i, "time", fmt.Sprintf("invalid time %02d:%02d", row.Hour, row.Minute))
		}
		if lastMinute > minuteOfDay {
			result.add(SeverityWarning, 0, "", i, "time_order", "minute-time rows are not sorted")
		}
		lastMinute = minuteOfDay
		if row.Price.Mantissa < 0 {
			result.add(SeverityError, 0, "", i, "price", "price is negative")
		}
		if row.Volume < 0 {
			result.add(SeverityWarning, 0, "", i, "volume", "volume is negative; minute volume field needs live fixture confirmation")
		}
		if len(row.Raw) == 0 {
			result.add(SeverityWarning, 0, "", i, "raw", "raw minute-time record is empty")
		}
	}
	return result.finalize()
}

func ValidateTransactions(operation string, rows []model.Transaction) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(rows)}
	if len(rows) == 0 {
		result.add(SeverityWarning, 0, "", 0, "rows", "empty transaction response")
		return result.finalize()
	}
	for i, row := range rows {
		if row.Hour < 0 || row.Hour > 23 || row.Minute < 0 || row.Minute > 59 {
			result.add(SeverityError, 0, "", i, "time", fmt.Sprintf("invalid time %02d:%02d", row.Hour, row.Minute))
		}
		if row.Price.Mantissa < 0 || row.Vol < 0 {
			result.add(SeverityError, 0, "", i, "price_volume", "price or volume is negative")
		}
		if len(row.Raw) == 0 {
			result.add(SeverityWarning, 0, "", i, "raw", "raw transaction record is empty")
		}
	}
	return result.finalize()
}

func ValidateFinanceInfo(operation string, info model.FinanceInfo) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: 1}
	if len(info.Code) != 6 {
		result.add(SeverityError, info.Market, info.Code, 0, "code", "finance code must be 6 characters")
	}
	if len(info.Raw) == 0 {
		result.add(SeverityWarning, info.Market, info.Code, 0, "raw", "raw finance record is empty")
	}
	return result.finalize()
}

func ValidateMarketStat(operation string, stat model.MarketStat) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: 1}
	if stat.TotalCount < 0 || stat.UpCount < 0 || stat.DownCount < 0 || stat.NeutralCount < 0 || stat.SuspendedCount < 0 {
		result.add(SeverityError, 0, "", 0, "counts", "market stat contains negative count")
	}
	if stat.TotalAmount < 0 || stat.TotalVolume < 0 {
		result.add(SeverityError, 0, "", 0, "amount_volume", "market stat amount or volume is negative")
	}
	return result.finalize()
}

func ValidateFundFlow(operation string, market model.Market, code string, flow model.FundFlow) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: 1}
	values := map[string]float64{
		"super_in":   flow.SuperIn,
		"large_in":   flow.LargeIn,
		"medium_in":  flow.MediumIn,
		"small_in":   flow.SmallIn,
		"super_out":  flow.SuperOut,
		"large_out":  flow.LargeOut,
		"medium_out": flow.MediumOut,
		"small_out":  flow.SmallOut,
	}
	for field, value := range values {
		if value < 0 {
			result.add(SeverityError, market, code, 0, field, "fund flow amount is negative")
		}
	}
	return result.finalize()
}

func ValidateHistoricalFundFlow(operation string, market model.Market, code string, rows []model.HistoricalFundFlow) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(rows)}
	if len(rows) == 0 {
		result.add(SeverityWarning, market, code, 0, "rows", "empty historical fund flow response")
		return result.finalize()
	}
	for i, row := range rows {
		if row.Year <= 0 || row.Month < 1 || row.Month > 12 || row.Day < 1 || row.Day > 31 {
			result.add(SeverityError, market, code, i, "date", fmt.Sprintf("invalid date %04d-%02d-%02d", row.Year, row.Month, row.Day))
		}
		values := map[string]float64{
			"super_in":   row.SuperIn,
			"large_in":   row.LargeIn,
			"medium_in":  row.MediumIn,
			"small_in":   row.SmallIn,
			"super_out":  row.SuperOut,
			"large_out":  row.LargeOut,
			"medium_out": row.MediumOut,
			"small_out":  row.SmallOut,
		}
		for field, value := range values {
			if value < 0 {
				result.add(SeverityError, market, code, i, field, "historical fund flow amount is negative")
			}
		}
		if len(row.Raw) == 0 {
			result.add(SeverityWarning, market, code, i, "raw", "raw historical fund flow record is empty")
		}
	}
	return result.finalize()
}

func ValidateXdxr(operation string, rows []model.XdxrRecord) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(rows)}
	for i, row := range rows {
		if len(row.Code) != 6 {
			result.add(SeverityError, row.Market, row.Code, i, "code", "xdxr code must be 6 characters")
		}
		if row.Year <= 0 || row.Month < 1 || row.Month > 12 || row.Day < 1 || row.Day > 31 {
			result.add(SeverityError, row.Market, row.Code, i, "date", fmt.Sprintf("invalid date %04d-%02d-%02d", row.Year, row.Month, row.Day))
		}
		if len(row.Raw) == 0 {
			result.add(SeverityWarning, row.Market, row.Code, i, "raw", "raw xdxr record is empty")
		}
	}
	return result.finalize()
}

func ValidateBoards(operation string, boards []model.Board) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(boards)}
	if len(boards) == 0 {
		result.add(SeverityWarning, 0, "", 0, "rows", "empty board response")
		return result.finalize()
	}
	for i, board := range boards {
		if board.Name == "" {
			result.add(SeverityError, 0, "", i, "name", "board name is empty")
		}
		if int(board.Count) != len(board.Codes) {
			result.add(SeverityError, 0, board.Name, i, "count", fmt.Sprintf("board count = %d, codes = %d", board.Count, len(board.Codes)))
		}
		for _, code := range board.Codes {
			if len(code) != 6 {
				result.add(SeverityWarning, 0, code, i, "code", "board member code is not 6 characters")
			}
		}
		if len(board.Raw) == 0 {
			result.add(SeverityWarning, 0, board.Name, i, "raw", "raw board record is empty")
		}
	}
	return result.finalize()
}

func ValidateCompanyInfoCategories(operation string, market model.Market, code string, rows []model.CompanyInfoCategory) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(rows)}
	if len(rows) == 0 {
		result.add(SeverityWarning, market, code, 0, "rows", "empty company info category response")
		return result.finalize()
	}
	for i, row := range rows {
		if row.Name == "" {
			result.add(SeverityError, market, code, i, "name", "company category name is empty")
		}
		if row.Filename == "" {
			result.add(SeverityError, market, code, i, "filename", "company category filename is empty")
		}
		if row.Start < 0 || row.Length < 0 {
			result.add(SeverityError, market, code, i, "range", "company category range contains negative value")
		}
		if len(row.Raw) == 0 {
			result.add(SeverityWarning, market, code, i, "raw", "raw company category record is empty")
		}
	}
	return result.finalize()
}

func ValidateBytes(operation string, data []byte, minLen int) CheckResult {
	result := CheckResult{Operation: operation, OK: true, Rows: len(data)}
	if len(data) < minLen {
		result.add(SeverityError, 0, "", 0, "bytes", fmt.Sprintf("byte length = %d, want at least %d", len(data), minLen))
	}
	return result.finalize()
}

func (r *CheckResult) add(severity Severity, market model.Market, code string, index int, field string, message string) {
	r.Findings = append(r.Findings, Finding{
		Severity:  severity,
		Operation: r.Operation,
		Market:    market,
		Code:      code,
		Index:     index,
		Field:     field,
		Message:   message,
	})
}

func (r CheckResult) finalize() CheckResult {
	r.OK = !hasError(r.Findings)
	return r
}

func hasError(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity != SeverityWarning {
			return true
		}
	}
	return false
}

func decimalLess(a, b model.Decimal) bool {
	return a.Float64() < b.Float64()
}

func symbolKey(market model.Market, code string) string {
	return market.String() + ":" + code
}
