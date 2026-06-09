package validation

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/model"
)

func TestLiveIntegrityReport(t *testing.T) {
	if os.Getenv("TDX_LIVE") != "1" {
		t.Skip("set TDX_LIVE=1 to run live TDX integrity validation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := tdx.NewClient(tdx.Options{Timeout: 8 * time.Second})
	defer client.Close()

	report := RunLive(ctx, client, LiveOptions{
		Markets:              []model.Market{model.MarketSH, model.MarketSZ},
		Symbols:              []model.Symbol{{Market: model.MarketSH, Code: "600519"}, {Market: model.MarketSZ, Code: "000001"}},
		KlineCategories:      []model.KlineCategory{model.KlineDay},
		BarCount:             10,
		TransactionCount:     50,
		HistoryFundFlowCount: 10,
		SkipBoards:           true,
		SkipReportFiles:      true,
	})
	if report.Summary.Errors > 0 {
		data, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("live integrity report has errors:\n%s", data)
	}
}
