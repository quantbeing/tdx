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

func TestRunOperationMatrixKeepsNamedVariantsSeparate(t *testing.T) {
	server := model.Server{Name: "good", Host: "127.0.0.1", Port: 7709}
	report := RunOperationMatrix(context.Background(), OperationMatrixOptions{
		Servers: []model.Server{server},
		Operations: []MatrixOperation{
			{Name: "security-list-sh", Command: command.NewSecurityListCommand(model.MarketSH, 0)},
			{Name: "security-list-bj", Command: command.NewSecurityListCommand(model.MarketBJ, 0)},
		},
		Repeats:             1,
		PerOperationTimeout: time.Second,
		NewClient: func(server model.Server, observer tdx.Observer) OperationMatrixClient {
			return &fakeMatrixClient{server: server, observer: observer}
		},
	})

	if len(report.Summary) != 2 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if report.Summary[0].Name != "security-list-bj" || report.Summary[1].Name != "security-list-sh" {
		t.Fatalf("summary names = %+v", report.Summary)
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

func TestRunOperationMatrixAddsTimeoutRecommendations(t *testing.T) {
	report := RunOperationMatrix(context.Background(), OperationMatrixOptions{
		Servers: []model.Server{
			{Name: "fast", Host: "127.0.0.1", Port: 7709},
			{Name: "timeout", Host: "127.0.0.2", Port: 7709},
		},
		Operations:          []MatrixOperation{{Name: "count", Command: command.NewSecurityCountCommand(model.MarketSH)}},
		Repeats:             1,
		PerOperationTimeout: 5 * time.Second,
		TimeoutRecommendation: TimeoutRecommendationOptions{
			MinTimeout:         500 * time.Millisecond,
			MaxTimeout:         2 * time.Second,
			SuccessMultiplier:  4,
			FailureFastTimeout: 1200 * time.Millisecond,
		},
		NewClient: func(server model.Server, observer tdx.Observer) OperationMatrixClient {
			return &fakeMatrixClient{
				server:   server,
				observer: observer,
				fail:     server.Name == "timeout",
			}
		},
	})

	if len(report.TimeoutRecommendations) != 2 {
		t.Fatalf("recommendations = %+v", report.TimeoutRecommendations)
	}
	if report.TimeoutRecommendations[0].RecommendedTimeoutMS != 500 || report.TimeoutRecommendations[0].Reason != "observed_success_latency" {
		t.Fatalf("success recommendation = %+v", report.TimeoutRecommendations[0])
	}
	if report.TimeoutRecommendations[1].RecommendedTimeoutMS != 500 || report.TimeoutRecommendations[1].Reason != "no_success_fail_fast" {
		t.Fatalf("failure recommendation = %+v", report.TimeoutRecommendations[1])
	}
}

func TestRecommendOperationTimeoutsUsesFailureFastForSlowFailures(t *testing.T) {
	recs := RecommendOperationTimeouts([]OperationMatrixSummary{{
		Name:         "security-list-bj",
		Operation:    "security_list",
		Server:       model.Server{Host: "127.0.0.1", Port: 7709},
		Runs:         2,
		Failures:     2,
		MaxLatencyMS: 2001,
	}}, TimeoutRecommendationOptions{
		MinTimeout:         500 * time.Millisecond,
		MaxTimeout:         3 * time.Second,
		FailureFastTimeout: 1200 * time.Millisecond,
	})

	if len(recs) != 1 || recs[0].RecommendedTimeoutMS != 1200 {
		t.Fatalf("recommendations = %+v", recs)
	}
}
