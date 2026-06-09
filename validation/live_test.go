package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/quantbeing/tdx/model"
)

func TestRunLiveBuildsIntegrityReportFromClientData(t *testing.T) {
	client := fakeLiveClient{}
	report := RunLive(context.Background(), client, LiveOptions{
		Markets:          []model.Market{model.MarketSH},
		Symbols:          []model.Symbol{{Market: model.MarketSH, Code: "600519"}},
		KlineCategories:  []model.KlineCategory{model.KlineDay},
		BarCount:         2,
		TransactionCount: 2,
		BoardTypes:       []string{"concept"},
		ReportFiles:      []string{"base_info.zip"},
	})

	if report.Summary.TotalResults == 0 {
		t.Fatalf("empty report: %+v", report)
	}
	if report.Summary.Errors != 0 || report.Summary.FailedResults != 0 {
		t.Fatalf("report has failures: %+v", report)
	}
	if report.FinishedAt.IsZero() {
		t.Fatalf("report not finished: %+v", report)
	}
}

func TestRunLiveContinuesAfterOperationError(t *testing.T) {
	client := fakeLiveClient{securityListErr: errors.New("timeout")}
	report := RunLive(context.Background(), client, LiveOptions{
		Markets: []model.Market{model.MarketSH},
		Symbols: []model.Symbol{{Market: model.MarketSH, Code: "600519"}},
	})

	if report.Summary.Errors == 0 || report.Summary.FailedResults == 0 {
		t.Fatalf("report did not keep operation error: %+v", report)
	}
	if report.Summary.TotalResults < 2 {
		t.Fatalf("report stopped too early: %+v", report)
	}
}

func TestRunLiveUsesPerOperationTimeout(t *testing.T) {
	client := fakeLiveClient{securityListWaitForContext: true}
	report := RunLive(context.Background(), client, LiveOptions{
		Markets:             []model.Market{model.MarketSH},
		Symbols:             []model.Symbol{{Market: model.MarketSH, Code: "600519"}},
		PerOperationTimeout: time.Millisecond,
		SkipBoards:          true,
		SkipReportFiles:     true,
	})

	if report.Summary.Errors == 0 {
		t.Fatalf("report did not record timeout: %+v", report)
	}
	if report.Summary.TotalResults < 3 {
		t.Fatalf("report stopped after timeout: %+v", report)
	}
}

func TestRunLiveCanValidateFullSecurityListPagination(t *testing.T) {
	client := fakeLiveClient{
		securityCount: 1001,
		securityPages: map[int][]model.Security{
			0:    makeSecurities(model.MarketSH, 0, 1000),
			1000: makeSecurities(model.MarketSH, 1000, 1),
		},
	}
	report := RunLive(context.Background(), client, LiveOptions{
		Markets:          []model.Market{model.MarketSH},
		Symbols:          []model.Symbol{{Market: model.MarketSH, Code: "600519"}},
		FullSecurityList: true,
		SkipBoards:       true,
		SkipReportFiles:  true,
	})

	result, ok := findResult(report, "security_list_SH_full")
	if !ok {
		t.Fatalf("missing full security list result: %+v", report.Results)
	}
	if !result.OK || result.Rows != 1001 {
		t.Fatalf("full security list result = %+v", result)
	}
}

func TestRunLiveFullSecurityListPreservesPartialRowsOnError(t *testing.T) {
	client := fakeLiveClient{
		securityCount:          1001,
		securityListErrAtStart: 1000,
		securityPages: map[int][]model.Security{
			0: makeSecurities(model.MarketSH, 0, 1000),
		},
	}
	report := RunLive(context.Background(), client, LiveOptions{
		Markets:          []model.Market{model.MarketSH},
		Symbols:          []model.Symbol{{Market: model.MarketSH, Code: "600519"}},
		FullSecurityList: true,
		SkipBoards:       true,
		SkipReportFiles:  true,
	})

	result, ok := findResult(report, "security_list_SH_full")
	if !ok {
		t.Fatalf("missing full security list result: %+v", report.Results)
	}
	if result.OK || result.Rows != 1000 {
		t.Fatalf("partial full security list result = %+v", result)
	}
}

type fakeLiveClient struct {
	securityListErr            error
	securityListErrAtStart     int
	securityListWaitForContext bool
	securityCount              uint16
	securityPages              map[int][]model.Security
}

func (f fakeLiveClient) GetSecurityCount(context.Context, model.Market) (uint16, error) {
	if f.securityCount > 0 {
		return f.securityCount, nil
	}
	return 2, nil
}

func (f fakeLiveClient) GetSecurityList(ctx context.Context, _ model.Market, start int) ([]model.Security, error) {
	if f.securityListWaitForContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.securityListErr != nil {
		return nil, f.securityListErr
	}
	if f.securityListErrAtStart > 0 && start == f.securityListErrAtStart {
		return nil, errors.New("page timeout")
	}
	if f.securityPages != nil {
		return f.securityPages[start], nil
	}
	return []model.Security{
		{Market: model.MarketSH, Code: "600519", Name: "贵州茅台", DecimalPoint: 2, Raw: make([]byte, 29)},
	}, nil
}

func makeSecurities(market model.Market, start int, count int) []model.Security {
	items := make([]model.Security, count)
	for i := range items {
		items[i] = model.Security{
			Market:       market,
			Code:         makeCode(start + i),
			Name:         "SEC",
			DecimalPoint: 2,
			Raw:          make([]byte, 29),
		}
	}
	return items
}

func makeCode(i int) string {
	return fmt.Sprintf("%06d", i)
}

func findResult(report Report, operation string) (CheckResult, bool) {
	for _, result := range report.Results {
		if result.Operation == operation {
			return result, true
		}
	}
	return CheckResult{}, false
}

func (fakeLiveClient) GetBars(context.Context, model.Market, string, model.KlineCategory, int, int) ([]model.Bar, error) {
	return []model.Bar{{
		Market: model.MarketSH, Code: "600519", Category: model.KlineDay,
		Open: model.NewPriceFromMilli(10000), Close: model.NewPriceFromMilli(10500),
		High: model.NewPriceFromMilli(10600), Low: model.NewPriceFromMilli(9900),
		Year: 2026, Month: 6, Day: 9, Vol: 100, Amount: 1000, Raw: []byte{1},
	}}, nil
}

func (fakeLiveClient) GetSecurityQuotes(context.Context, []model.Symbol) ([]model.Quote, error) {
	return []model.Quote{{
		Market: model.MarketSH, Code: "600519", Price: model.NewPriceFromMilli(10500),
		Vol: 100, Amount: 1000, Raw: []byte{1},
	}}, nil
}

func (fakeLiveClient) GetMinuteTimeData(context.Context, model.Market, string) ([]model.MinuteTime, error) {
	return []model.MinuteTime{{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(10500), Volume: 10, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) GetTransactionData(context.Context, model.Market, string, int, int) ([]model.Transaction, error) {
	return []model.Transaction{{Hour: 9, Minute: 31, Price: model.NewPriceFromMilli(10500), Vol: 1, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) GetMarketStat(context.Context) (model.MarketStat, error) {
	return model.MarketStat{TotalCount: 10, TotalAmount: 100, TotalVolume: 20}, nil
}

func (fakeLiveClient) GetFundFlow(context.Context, model.Market, string) (model.FundFlow, error) {
	return model.FundFlow{SuperIn: 1, SuperOut: 1}, nil
}

func (fakeLiveClient) GetHistoryFundFlow(context.Context, model.Market, string, int, int) ([]model.HistoricalFundFlow, error) {
	return []model.HistoricalFundFlow{{Year: 2026, Month: 6, Day: 9, SuperIn: 1, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) GetFinanceInfo(context.Context, model.Market, string) (model.FinanceInfo, error) {
	return model.FinanceInfo{Market: model.MarketSH, Code: "600519", Raw: []byte{1}}, nil
}

func (fakeLiveClient) GetXdxrInfo(context.Context, model.Market, string) ([]model.XdxrRecord, error) {
	return []model.XdxrRecord{{Market: model.MarketSH, Code: "600519", Year: 2026, Month: 6, Day: 9, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) GetCompanyInfoCategory(context.Context, model.Market, string) ([]model.CompanyInfoCategory, error) {
	return []model.CompanyInfoCategory{{Name: "profile", Filename: "600519.txt", Length: 10, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) ListBoards(context.Context, string) ([]model.Board, error) {
	return []model.Board{{Name: "concept", Count: 1, Codes: []string{"600519"}, Raw: []byte{1}}}, nil
}

func (fakeLiveClient) GetReportFile(context.Context, string) ([]byte, error) {
	return []byte{1, 2, 3}, nil
}
