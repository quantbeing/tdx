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
)

func main() {
	if err := run(os.Args[1:], os.Stdout, nil, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer, capturer diagnostic.Capturer, getenv func(string) string) error {
	var outDir string
	var opsRaw string
	var timeout time.Duration
	var allowLive bool
	var maxAttempts int
	var retryStrategyRaw string
	var sameHostAttempts int
	fs := flag.NewFlagSet("tdx-fixture-matrix", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&outDir, "out", "fixtures/live", "fixture output directory")
	fs.StringVar(&opsRaw, "ops", "", "comma-separated operation names; empty means all")
	fs.DurationVar(&timeout, "timeout", 20*time.Second, "overall capture timeout")
	fs.BoolVar(&allowLive, "allow-live", false, "run without TDX_LIVE=1")
	fs.IntVar(&maxAttempts, "max-attempts", 0, "maximum attempts per capture command; 0 uses client default")
	fs.StringVar(&retryStrategyRaw, "retry-strategy", "failover-first", "retry strategy: failover-first or same-host-first")
	fs.IntVar(&sameHostAttempts, "same-host-attempts", 1, "same-host attempts for same-host-first retry strategy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if getenv("TDX_LIVE") != "1" && !allowLive {
		return fmt.Errorf("refusing live TDX capture without TDX_LIVE=1")
	}
	names := splitOps(opsRaw)
	ops, unknown := diagnostic.SelectMatrixOperations(diagnostic.DefaultMatrixOperations(), names)
	if len(unknown) > 0 {
		return fmt.Errorf("unknown operations: %s", strings.Join(unknown, ","))
	}
	if capturer == nil {
		clientOpts, err := buildClientOptions(timeout, maxAttempts, retryStrategyRaw, sameHostAttempts)
		if err != nil {
			return err
		}
		client := tdx.NewClient(clientOpts)
		defer client.Close()
		capturer = client
	} else if _, err := parseRetryStrategy(retryStrategyRaw); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := diagnostic.RunCaptureMatrix(ctx, capturer, outDir, ops)
	enc := json.NewEncoder(stdout)
	for _, result := range results {
		if err := enc.Encode(result); err != nil {
			return err
		}
	}
	return nil
}

func buildClientOptions(timeout time.Duration, maxAttempts int, retryStrategyRaw string, sameHostAttempts int) (tdx.Options, error) {
	retryStrategy, err := parseRetryStrategy(retryStrategyRaw)
	if err != nil {
		return tdx.Options{}, err
	}
	return tdx.Options{
		Timeout:     timeout,
		MaxAttempts: maxAttempts,
		Retry: tdx.RetryOptions{
			Strategy:         retryStrategy,
			SameHostAttempts: sameHostAttempts,
		},
	}, nil
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

func splitOps(raw string) []string {
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
