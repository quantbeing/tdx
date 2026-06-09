package diagnostic

import (
	"context"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

type Capturer interface {
	Capture(context.Context, command.Command) (tdx.CapturedResponse, error)
}

type MatrixOperation struct {
	Name    string
	Command command.Command
}

type MatrixResult struct {
	Name        string       `json:"name"`
	Operation   string       `json:"operation"`
	OK          bool         `json:"ok"`
	Path        string       `json:"path,omitempty"`
	Server      model.Server `json:"server,omitempty"`
	LatencyMS   int64        `json:"latency_ms,omitempty"`
	RawBodySize int          `json:"raw_body_size,omitempty"`
	BodySize    int          `json:"body_size,omitempty"`
	Error       string       `json:"error,omitempty"`
}

func DefaultMatrixOperations() []MatrixOperation {
	return []MatrixOperation{
		{Name: "security-count", Command: command.NewSecurityCountCommand(model.MarketSH)},
		{Name: "security-list-sh", Command: command.NewSecurityListCommand(model.MarketSH, 0)},
		{Name: "security-list-sz", Command: command.NewSecurityListCommand(model.MarketSZ, 0)},
		{Name: "security-list-bj", Command: command.NewSecurityListCommand(model.MarketBJ, 0)},
		{Name: "stock-bars", Command: command.NewSecurityBarsCommand(model.MarketSH, "600519", model.KlineDay, 0, 10)},
		{Name: "index-bars", Command: command.NewIndexBarsCommand(model.MarketSH, "000001", model.KlineDay, 0, 10)},
		{Name: "quote", Command: command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "600519"}})},
		{Name: "market-stat-source", Command: command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "880005"}})},
		{Name: "minute", Command: command.NewMinuteTimeDataCommand(model.MarketSH, "600519")},
		{Name: "transaction", Command: command.NewTransactionDataCommand(model.MarketSH, "600519", 0, 50)},
		{Name: "history-fund-flow", Command: command.NewHistoryFundFlowCommand(model.MarketSH, "600519", 0, 10)},
		{Name: "finance", Command: command.NewFinanceInfoCommand(model.MarketSH, "600519")},
		{Name: "xdxr", Command: command.NewXdxrInfoCommand(model.MarketSH, "600519")},
		{Name: "company", Command: command.NewCompanyInfoCategoryCommand(model.MarketSH, "600519")},
		{Name: "block-meta", Command: command.NewBlockInfoMetaCommand("block_gn.dat")},
		{Name: "block", Command: command.NewBlockInfoCommand("block_gn.dat", 0, tdx.DefaultFileChunkSize)},
		{Name: "report", Command: command.NewReportFileCommand("base_info.zip", 0, tdx.DefaultFileChunkSize)},
	}
}

func SelectMatrixOperations(all []MatrixOperation, names []string) ([]MatrixOperation, []string) {
	if len(names) == 0 {
		return append([]MatrixOperation(nil), all...), nil
	}
	byName := make(map[string]MatrixOperation, len(all))
	for _, op := range all {
		byName[op.Name] = op
	}
	selected := make([]MatrixOperation, 0, len(names))
	unknown := make([]string, 0)
	for _, name := range names {
		op, ok := byName[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		selected = append(selected, op)
	}
	return selected, unknown
}

func RunCaptureMatrix(ctx context.Context, capturer Capturer, dir string, ops []MatrixOperation) []MatrixResult {
	results := make([]MatrixResult, 0, len(ops))
	for _, op := range ops {
		result := MatrixResult{Name: op.Name, Operation: op.Command.Operation()}
		start := time.Now()
		capture, err := capturer.Capture(ctx, op.Command)
		result.LatencyMS = time.Since(start).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		summary, err := WriteFixture(DefaultFixturePath(dir, capture), capture)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		result.OK = true
		result.Path = summary.Path
		result.Server = summary.Server
		result.RawBodySize = summary.RawBodySize
		result.BodySize = summary.BodySize
		if capture.Latency > 0 {
			result.LatencyMS = capture.Latency.Milliseconds()
		}
		results = append(results, result)
	}
	return results
}
