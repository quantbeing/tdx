package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/quantbeing/tdx/model"
)

func TestRunRequiresLiveOptIn(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-markets", "sh", "-symbols", "sh:600519"}, &out, fakeValidateClient{}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "TDX_LIVE=1") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWritesIntegrityReportJSON(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"-markets", "sh",
		"-symbols", "sh:600519",
		"-kline", "day",
		"-skip-boards",
		"-skip-files",
	}, &out, fakeValidateClient{}, func(string) string { return "1" })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, `"total_results"`) || !strings.Contains(text, `"security_count_SH"`) || !strings.Contains(text, `"security_quotes"`) {
		t.Fatalf("output = %s", text)
	}
}

func TestRunCanEnableFullSecurityListValidation(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"-markets", "sh",
		"-symbols", "sh:600519",
		"-skip-boards",
		"-skip-files",
		"-full-security-list",
	}, &out, fakeValidateClient{}, func(string) string { return "1" })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"security_list_SH_full"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestParseSymbolsRejectsBadToken(t *testing.T) {
	_, err := parseSymbols("sh600519")
	if err == nil || !strings.Contains(err.Error(), "market:code") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildClientOptionsUsesShortTransportTimeouts(t *testing.T) {
	opts := buildClientOptions(60*time.Second, 5*time.Second, 2*time.Second)
	if opts.Timeout != 5*time.Second {
		t.Fatalf("timeout = %s, want 5s", opts.Timeout)
	}
	if opts.Transport.ConnectTimeout != 2*time.Second || opts.Transport.WriteTimeout != 2*time.Second || opts.Transport.ReadTimeout != 5*time.Second {
		t.Fatalf("transport = %+v", opts.Transport)
	}
}

type fakeValidateClient struct{}

func (fakeValidateClient) GetSecurityCount(context.Context, model.Market) (uint16, error) {
	return 1, nil
}

func (fakeValidateClient) GetSecurityList(context.Context, model.Market, int) ([]model.Security, error) {
	return []model.Security{{Market: model.MarketSH, Code: "600519", Name: "贵州茅台", DecimalPoint: 2, Raw: make([]byte, 29)}}, nil
}

func (fakeValidateClient) GetBars(context.Context, model.Market, string, model.KlineCategory, int, int) ([]model.Bar, error) {
	return []model.Bar{{
		Market: model.MarketSH, Code: "600519", Category: model.KlineDay,
		Open: model.NewPriceFromMilli(10000), Close: model.NewPriceFromMilli(10500),
		High: model.NewPriceFromMilli(10600), Low: model.NewPriceFromMilli(9900),
		Year: 2026, Month: 6, Day: 9, Vol: 100, Amount: 1000, Raw: []byte{1},
	}}, nil
}

func (fakeValidateClient) GetSecurityQuotes(context.Context, []model.Symbol) ([]model.Quote, error) {
	return []model.Quote{{Market: model.MarketSH, Code: "600519", Price: model.NewPriceFromMilli(10500), Vol: 1, Amount: 1, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetMinuteTimeData(context.Context, model.Market, string) ([]model.MinuteTime, error) {
	return []model.MinuteTime{{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(10500), Volume: 1, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetTransactionData(context.Context, model.Market, string, int, int) ([]model.Transaction, error) {
	return []model.Transaction{{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(10500), Vol: 1, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetMarketStat(context.Context) (model.MarketStat, error) {
	return model.MarketStat{TotalCount: 1, TotalAmount: 1, TotalVolume: 1}, nil
}

func (fakeValidateClient) GetFundFlow(context.Context, model.Market, string) (model.FundFlow, error) {
	return model.FundFlow{}, nil
}

func (fakeValidateClient) GetHistoryFundFlow(context.Context, model.Market, string, int, int) ([]model.HistoricalFundFlow, error) {
	return []model.HistoricalFundFlow{{Year: 2026, Month: 6, Day: 9, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetFinanceInfo(context.Context, model.Market, string) (model.FinanceInfo, error) {
	return model.FinanceInfo{Market: model.MarketSH, Code: "600519", Raw: []byte{1}}, nil
}

func (fakeValidateClient) GetXdxrInfo(context.Context, model.Market, string) ([]model.XdxrRecord, error) {
	return []model.XdxrRecord{{Market: model.MarketSH, Code: "600519", Year: 2026, Month: 6, Day: 9, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetCompanyInfoCategory(context.Context, model.Market, string) ([]model.CompanyInfoCategory, error) {
	return []model.CompanyInfoCategory{{Name: "profile", Filename: "600519.txt", Length: 10, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) ListBoards(context.Context, string) ([]model.Board, error) {
	return []model.Board{{Name: "concept", Count: 1, Codes: []string{"600519"}, Raw: []byte{1}}}, nil
}

func (fakeValidateClient) GetReportFile(context.Context, string) ([]byte, error) {
	return []byte{1}, nil
}
