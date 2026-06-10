package diagnostic

import (
	"context"
	"errors"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

type fakeMatrixClient struct {
	server   model.Server
	observer tdx.Observer
	fail     bool
}

func (f *fakeMatrixClient) HealthCheck(_ context.Context, ops ...command.Command) []tdx.OperationHealth {
	out := make([]tdx.OperationHealth, 0, len(ops))
	for _, op := range ops {
		if f.fail {
			if f.observer != nil {
				f.observer.OnRequest(tdx.RequestEvent{
					Operation: op.Operation(),
					Server:    f.server,
					Attempt:   1,
					OK:        false,
					Error:     "timeout",
					Latency:   5 * time.Millisecond,
				})
			}
			out = append(out, tdx.OperationHealth{Operation: op.Operation(), OK: false, Latency: 5 * time.Millisecond, Error: "timeout"})
			continue
		}
		if f.observer != nil {
			f.observer.OnRequest(tdx.RequestEvent{
				Operation: op.Operation(),
				Server:    f.server,
				Attempt:   1,
				OK:        true,
				Latency:   2 * time.Millisecond,
				Rows:      7,
				BodySize:  11,
			})
		}
		out = append(out, tdx.OperationHealth{Operation: op.Operation(), OK: true, Latency: 2 * time.Millisecond})
	}
	return out
}

func (f *fakeMatrixClient) Close() error {
	return nil
}

func TestRunOperationMatrixAggregatesByOperationAndHost(t *testing.T) {
	servers := []model.Server{
		{Name: "good", Host: "127.0.0.1", Port: 7709},
		{Name: "bad", Host: "127.0.0.2", Port: 7709},
	}
	ops := []MatrixOperation{{Name: "count", Command: command.NewSecurityCountCommand(model.MarketSH)}}

	report := RunOperationMatrix(context.Background(), OperationMatrixOptions{
		Servers:             servers,
		Operations:          ops,
		Repeats:             2,
		PerOperationTimeout: time.Second,
		NewClient: func(server model.Server, observer tdx.Observer) OperationMatrixClient {
			return &fakeMatrixClient{
				server:   server,
				observer: observer,
				fail:     server.Name == "bad",
			}
		},
	})

	if len(report.Results) != 4 {
		t.Fatalf("results = %+v", report.Results)
	}
	if len(report.Summary) != 2 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if report.Summary[0].Server.Addr() != "127.0.0.1:7709" || report.Summary[0].Successes != 2 || report.Summary[0].Failures != 0 {
		t.Fatalf("good summary = %+v", report.Summary[0])
	}
	if report.Summary[1].Server.Addr() != "127.0.0.2:7709" || report.Summary[1].Successes != 0 || report.Summary[1].Failures != 2 {
		t.Fatalf("bad summary = %+v", report.Summary[1])
	}
	if report.Summary[0].SuccessRate != 1 || report.Summary[1].SuccessRate != 0 {
		t.Fatalf("success rates = %+v", report.Summary)
	}
	if len(report.Results[0].Attempts) != 1 || report.Results[0].Attempts[0].Rows != 7 || report.Results[0].Attempts[0].BodySize != 11 {
		t.Fatalf("attempts = %+v", report.Results[0].Attempts)
	}
}

func TestRunOperationMatrixRecordsClientFactoryError(t *testing.T) {
	report := RunOperationMatrix(context.Background(), OperationMatrixOptions{
		Servers:             []model.Server{{Name: "bad", Host: "127.0.0.2", Port: 7709}},
		Operations:          []MatrixOperation{{Name: "count", Command: command.NewSecurityCountCommand(model.MarketSH)}},
		Repeats:             1,
		PerOperationTimeout: time.Second,
		NewClient: func(model.Server, tdx.Observer) OperationMatrixClient {
			return operationMatrixClientError{err: errors.New("boom")}
		},
	})

	if len(report.Results) != 1 || report.Results[0].OK || report.Results[0].Error != "boom" {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Summary) != 1 || report.Summary[0].Failures != 1 || report.Summary[0].LastError != "boom" {
		t.Fatalf("summary = %+v", report.Summary)
	}
}
