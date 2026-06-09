package validation

import (
	"testing"

	"github.com/quantbeing/tdx/model"
)

func TestValidateSecuritiesReportsMalformedRows(t *testing.T) {
	result := ValidateSecurities("security_list", model.MarketSH, []model.Security{
		{Market: model.MarketSH, Code: "600519", Name: "贵州茅台", DecimalPoint: 2, Raw: make([]byte, 29)},
		{Market: model.MarketSZ, Code: "1", Name: "", Raw: []byte{1}},
	})

	if result.OK {
		t.Fatalf("result unexpectedly OK: %+v", result)
	}
	if result.Rows != 2 {
		t.Fatalf("rows = %d, want 2", result.Rows)
	}
	if len(result.Findings) < 4 {
		t.Fatalf("findings = %+v, want malformed row findings", result.Findings)
	}
	if result.Findings[0].Operation != "security_list" || result.Findings[0].Index != 1 {
		t.Fatalf("first finding = %+v", result.Findings[0])
	}
}

func TestValidateQuotesChecksRequestedSymbolsAndRawRecords(t *testing.T) {
	requested := []model.Symbol{
		{Market: model.MarketSH, Code: "600519"},
		{Market: model.MarketSZ, Code: "000001"},
	}
	result := ValidateQuotes("security_quotes", requested, []model.Quote{
		{Market: model.MarketSH, Code: "600519", Price: model.NewPriceFromMilli(10000), Raw: []byte{1}},
		{Market: model.MarketSZ, Code: "000002", Price: model.NewPriceFromMilli(-1)},
	})

	if result.OK {
		t.Fatalf("result unexpectedly OK: %+v", result)
	}
	if len(result.Findings) < 3 {
		t.Fatalf("findings = %+v, want symbol/price/raw findings", result.Findings)
	}
}

func TestValidateMinuteTimesTreatsNegativeVolumeAsWarning(t *testing.T) {
	result := ValidateMinuteTimes("minute_time", []model.MinuteTime{
		{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(10000), Volume: -1, Raw: []byte{1}},
	})

	if !result.OK {
		t.Fatalf("result unexpectedly failed: %+v", result)
	}
	if len(result.Findings) != 1 || result.Findings[0].Severity != SeverityWarning {
		t.Fatalf("findings = %+v, want one warning", result.Findings)
	}
}

func TestValidateMinuteTimesRejectsNegativePrice(t *testing.T) {
	result := ValidateMinuteTimes("minute_time", []model.MinuteTime{
		{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(-1), Volume: 1, Raw: []byte{1}},
	})

	if result.OK {
		t.Fatalf("result unexpectedly OK: %+v", result)
	}
}

func TestReportSummaryCountsResultsAndFindings(t *testing.T) {
	report := NewReport()
	report.Add(CheckResult{Operation: "ok", OK: true, Rows: 2})
	report.Add(CheckResult{Operation: "bad", OK: false, Rows: 1, Findings: []Finding{
		{Severity: SeverityError, Message: "broken"},
		{Severity: SeverityWarning, Message: "suspicious"},
	}})

	report.Finish()

	if report.Summary.TotalResults != 2 || report.Summary.OKResults != 1 || report.Summary.FailedResults != 1 {
		t.Fatalf("summary counts = %+v", report.Summary)
	}
	if report.Summary.Errors != 1 || report.Summary.Warnings != 1 || report.Summary.Rows != 3 {
		t.Fatalf("summary findings = %+v", report.Summary)
	}
	if report.FinishedAt.IsZero() || report.DurationMS < 0 {
		t.Fatalf("finish metadata = finished=%s duration=%d", report.FinishedAt, report.DurationMS)
	}
}
