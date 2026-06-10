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
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/model"
)

const defaultOps = "security-count,quote,security-list-bj"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Getenv, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer, getenv func(string) string, factory diagnostic.OperationMatrixClientFactory) error {
	var hostsRaw string
	var opsRaw string
	var repeats int
	var timeout time.Duration
	var operationTimeout time.Duration
	var connectTimeout time.Duration
	var allowLive bool
	var jsonl bool

	fs := flag.NewFlagSet("tdx-op-matrix", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&hostsRaw, "hosts", "", "comma-separated host[:port] list; empty uses known servers")
	fs.StringVar(&opsRaw, "ops", defaultOps, "comma-separated operation names")
	fs.IntVar(&repeats, "repeats", 1, "number of times to run every operation on every host")
	fs.DurationVar(&timeout, "timeout", 60*time.Second, "overall matrix timeout")
	fs.DurationVar(&operationTimeout, "operation-timeout", 8*time.Second, "timeout per host/operation run")
	fs.DurationVar(&connectTimeout, "connect-timeout", time.Second, "TCP connect/write timeout")
	fs.BoolVar(&allowLive, "allow-live", false, "run without TDX_LIVE=1")
	fs.BoolVar(&jsonl, "jsonl", false, "write each result as JSONL followed by one summary line")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if getenv("TDX_LIVE") != "1" && !allowLive {
		return fmt.Errorf("refusing live TDX operation matrix without TDX_LIVE=1")
	}

	ops, unknown := diagnostic.SelectMatrixOperations(diagnostic.DefaultMatrixOperations(), splitCSV(opsRaw))
	if len(unknown) > 0 {
		return fmt.Errorf("unknown operations: %s", strings.Join(unknown, ","))
	}
	servers := parseServers(hostsRaw)
	if len(servers) == 0 {
		servers = tdx.KnownServers()
	}
	if factory == nil {
		factory = func(server model.Server, observer tdx.Observer) diagnostic.OperationMatrixClient {
			return tdx.NewClient(tdx.Options{
				Servers:     []model.Server{server},
				MaxAttempts: 1,
				Timeout:     operationTimeout,
				Transport: tdx.TransportOptions{
					ConnectTimeout: connectTimeout,
					WriteTimeout:   connectTimeout,
					ReadTimeout:    operationTimeout,
				},
				Pool:     tdx.PoolOptions{MaxIdlePerHost: 0},
				Observer: observer,
			})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	report := diagnostic.RunOperationMatrix(ctx, diagnostic.OperationMatrixOptions{
		Servers:             servers,
		Operations:          ops,
		Repeats:             repeats,
		PerOperationTimeout: operationTimeout,
		NewClient:           factory,
	})
	enc := json.NewEncoder(stdout)
	if jsonl {
		for _, result := range report.Results {
			if err := enc.Encode(result); err != nil {
				return err
			}
		}
		if err := enc.Encode(map[string]any{
			"summary":                 report.Summary,
			"timeout_recommendations": report.TimeoutRecommendations,
			"duration_ms":             report.DurationMS,
			"canceled":                report.Canceled,
			"error":                   report.Error,
		}); err != nil {
			return err
		}
		if report.Canceled {
			return fmt.Errorf("operation matrix canceled: %s", report.Error)
		}
		return nil
	}
	if err := enc.Encode(report); err != nil {
		return err
	}
	if report.Canceled {
		return fmt.Errorf("operation matrix canceled: %s", report.Error)
	}
	return nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
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
