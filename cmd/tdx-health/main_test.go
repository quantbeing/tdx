package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

func TestProbeCommandsSelectsNamedOperations(t *testing.T) {
	cmds, unknown := probeCommands("security-count,quote")
	if len(unknown) != 0 {
		t.Fatalf("unknown = %v", unknown)
	}
	if len(cmds) != 2 {
		t.Fatalf("len(cmds) = %d, want 2", len(cmds))
	}
	if cmds[0].Operation() != "security_count" || cmds[1].Operation() != "security_quotes" {
		t.Fatalf("operations = %s/%s", cmds[0].Operation(), cmds[1].Operation())
	}
}

func TestProbeCommandsReportsUnknownOperations(t *testing.T) {
	cmds, unknown := probeCommands("security-count,nope")
	if len(cmds) != 1 || len(unknown) != 1 || unknown[0] != "nope" {
		t.Fatalf("cmds=%d unknown=%v", len(cmds), unknown)
	}
}

func TestRunPingAllIgnoresRetryFlags(t *testing.T) {
	var out bytes.Buffer
	var called bool
	code := run([]string{
		"-hosts", "127.0.0.1:7709",
		"-retry-strategy", "sticky",
	}, &out, func(_ context.Context, servers []model.Server, _ tdx.TransportOptions) []tdx.PingResult {
		called = true
		return []tdx.PingResult{{Server: servers[0], Latency: time.Millisecond}}
	}, nil)
	if code != 0 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
	if !called {
		t.Fatal("pingAll was not called")
	}
	if !strings.Contains(out.String(), `"host":"127.0.0.1"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunProbePassesRetryOptions(t *testing.T) {
	var out bytes.Buffer
	var got tdx.Options
	code := run([]string{
		"-hosts", "127.0.0.1:7709",
		"-probe", "security-count",
		"-max-attempts", "3",
		"-retry-strategy", "same-host",
		"-same-host-attempts", "2",
	}, &out, nil, func(_ context.Context, opts tdx.Options, probes ...command.Command) (healthProbeClient, []tdx.HostHealth, error) {
		got = opts
		return fakeHealthProbeClient{stats: []model.ServerStat{{Server: opts.Servers[0]}}},
			[]tdx.HostHealth{{Server: opts.Servers[0], OK: true}},
			nil
	})
	if code != 0 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
	if got.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts = %d, want 3", got.MaxAttempts)
	}
	if got.Retry != (tdx.RetryOptions{Strategy: tdx.RetryStrategySameHostFirst, SameHostAttempts: 2}) {
		t.Fatalf("Retry = %+v", got.Retry)
	}
	if !strings.Contains(out.String(), `"ok":true`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunProbeRejectsUnknownRetryStrategy(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{
		"-probe", "security-count",
		"-retry-strategy", "sticky",
	}, &out, nil, nil)
	if code != 2 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "unknown retry strategy") {
		t.Fatalf("output = %s", out.String())
	}
}

type fakeHealthProbeClient struct {
	stats []model.ServerStat
}

func (c fakeHealthProbeClient) ServerStats() []model.ServerStat {
	return c.stats
}

func (fakeHealthProbeClient) Close() error {
	return nil
}
