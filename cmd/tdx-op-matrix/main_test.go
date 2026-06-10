package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/model"
)

type cliMatrixFakeClient struct {
	server   model.Server
	observer tdx.Observer
}

func (c cliMatrixFakeClient) HealthCheck(_ context.Context, ops ...command.Command) []tdx.OperationHealth {
	out := make([]tdx.OperationHealth, 0, len(ops))
	for _, op := range ops {
		if c.observer != nil {
			c.observer.OnRequest(tdx.RequestEvent{
				Operation: op.Operation(),
				Server:    c.server,
				Attempt:   1,
				OK:        true,
				Latency:   time.Millisecond,
				Rows:      1,
			})
		}
		out = append(out, tdx.OperationHealth{Operation: op.Operation(), OK: true, Latency: time.Millisecond})
	}
	return out
}

func (c cliMatrixFakeClient) Close() error {
	return nil
}

func TestRunWritesMatrixJSON(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"-allow-live",
		"-hosts", "127.0.0.1:7709",
		"-ops", "security-count,quote",
		"-repeats", "2",
	}, &out, func(string) string { return "" }, func(server model.Server, observer tdx.Observer) diagnostic.OperationMatrixClient {
		return cliMatrixFakeClient{server: server, observer: observer}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"results"`) || !strings.Contains(got, `"summary"`) {
		t.Fatalf("output = %s", got)
	}
	if !strings.Contains(got, `"runs":2`) || !strings.Contains(got, `"success_rate":1`) {
		t.Fatalf("output = %s", got)
	}
}

func TestRunWritesJSONL(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{
		"-allow-live",
		"-hosts", "127.0.0.1:7709",
		"-ops", "security-count",
		"-jsonl",
	}, &out, func(string) string { return "" }, func(server model.Server, observer tdx.Observer) diagnostic.OperationMatrixClient {
		return cliMatrixFakeClient{server: server, observer: observer}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", out.String())
	}
	if !strings.Contains(lines[0], `"name":"security-count"`) || !strings.Contains(lines[1], `"summary"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunRequiresLiveOptIn(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-hosts", "127.0.0.1:7709"}, &out, func(string) string { return "" }, nil)
	if err == nil || !strings.Contains(err.Error(), "TDX_LIVE=1") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunRejectsUnknownOperation(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-allow-live", "-ops", "missing"}, &out, func(string) string { return "" }, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown operations") {
		t.Fatalf("err = %v", err)
	}
}
