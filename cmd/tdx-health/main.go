package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/model"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, nil, nil))
}

type healthProbeClient interface {
	ServerStats() []model.ServerStat
	Close() error
}

type healthPingAllFunc func(context.Context, []model.Server, tdx.TransportOptions) []tdx.PingResult

type healthBestHostFunc func(context.Context, tdx.Options, ...command.Command) (healthProbeClient, []tdx.HostHealth, error)

func run(args []string, stdout io.Writer, pingAll healthPingAllFunc, fromBest healthBestHostFunc) int {
	var hosts string
	var probe string
	var timeout time.Duration
	var maxAttempts int
	var retryStrategyRaw string
	var sameHostAttempts int
	fs := flag.NewFlagSet("tdx-health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&hosts, "hosts", "", "comma-separated host[:port] list")
	fs.StringVar(&probe, "probe", "", "comma-separated diagnostic operation names, for example security-count,security-list-sh,quote")
	fs.DurationVar(&timeout, "timeout", 5*time.Second, "connect/read timeout")
	fs.IntVar(&maxAttempts, "max-attempts", 0, "maximum attempts per request; 0 uses client default")
	fs.StringVar(&retryStrategyRaw, "retry-strategy", "failover-first", "retry strategy: failover-first or same-host-first")
	fs.IntVar(&sameHostAttempts, "same-host-attempts", 1, "same-host attempts for same-host-first retry strategy")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	servers := parseServers(hosts)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	transport := tdx.TransportOptions{
		ConnectTimeout: timeout,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
	}
	if strings.TrimSpace(probe) != "" {
		retryStrategy, err := parseRetryStrategy(retryStrategyRaw)
		if err != nil {
			_ = json.NewEncoder(stdout).Encode(probeReport{OK: false, Error: err.Error()})
			return 2
		}
		cmds, unknown := probeCommands(probe)
		if len(unknown) > 0 {
			_ = json.NewEncoder(stdout).Encode(probeReport{OK: false, Error: "unknown probe operations", Unknown: unknown})
			return 2
		}
		if fromBest == nil {
			fromBest = func(ctx context.Context, opts tdx.Options, probes ...command.Command) (healthProbeClient, []tdx.HostHealth, error) {
				return tdx.FromBestHostByOperations(ctx, opts, probes...)
			}
		}
		client, health, err := fromBest(ctx, tdx.Options{
			Servers:     servers,
			Timeout:     timeout,
			MaxAttempts: maxAttempts,
			Retry: tdx.RetryOptions{
				Strategy:         retryStrategy,
				SameHostAttempts: sameHostAttempts,
			},
			Transport: transport,
		}, cmds...)
		report := probeReport{OK: err == nil, Health: health}
		if client != nil {
			stats := client.ServerStats()
			if len(stats) > 0 && err == nil {
				selected := stats[0].Server
				report.Selected = &selected
			}
			_ = client.Close()
		}
		if err != nil {
			report.Error = err.Error()
		}
		if encErr := json.NewEncoder(stdout).Encode(report); encErr != nil {
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}

	if pingAll == nil {
		pingAll = tdx.PingAll
	}
	results := pingAll(ctx, servers, transport)
	if err := json.NewEncoder(stdout).Encode(results); err != nil {
		return 1
	}
	return 0
}

type probeReport struct {
	OK       bool             `json:"ok"`
	Selected *model.Server    `json:"selected,omitempty"`
	Health   []tdx.HostHealth `json:"health,omitempty"`
	Unknown  []string         `json:"unknown,omitempty"`
	Error    string           `json:"error,omitempty"`
}

func probeCommands(raw string) ([]command.Command, []string) {
	names := splitCSV(raw)
	ops, unknown := diagnostic.SelectMatrixOperations(diagnostic.DefaultMatrixOperations(), names)
	cmds := make([]command.Command, 0, len(ops))
	for _, op := range ops {
		cmds = append(cmds, op.Command)
	}
	return cmds, unknown
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseRetryStrategy(raw string) (tdx.RetryStrategy, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "failover-first", "failover_first", "failover":
		return tdx.RetryStrategyFailoverFirst, nil
	case "same-host-first", "same_host_first", "same-host":
		return tdx.RetryStrategySameHostFirst, nil
	default:
		return "", fmt.Errorf("unknown retry strategy %q", raw)
	}
}

func parseServers(raw string) []model.Server {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]model.Server, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		host := part
		port := 7709
		if hp := strings.Split(part, ":"); len(hp) == 2 {
			host = hp[0]
			_, _ = fmt.Sscanf(hp[1], "%d", &port)
		}
		out = append(out, model.Server{Host: host, Port: port})
	}
	return out
}
