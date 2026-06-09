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
	client := tdx.NewClient(tdx.Options{})
	if err := run(os.Args[1:], os.Stdout, client, os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, stdout io.Writer, capturer diagnostic.Capturer, getenv func(string) string) error {
	var outDir string
	var opsRaw string
	var timeout time.Duration
	var allowLive bool
	fs := flag.NewFlagSet("tdx-fixture-matrix", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&outDir, "out", "fixtures/live", "fixture output directory")
	fs.StringVar(&opsRaw, "ops", "", "comma-separated operation names; empty means all")
	fs.DurationVar(&timeout, "timeout", 20*time.Second, "overall capture timeout")
	fs.BoolVar(&allowLive, "allow-live", false, "run without TDX_LIVE=1")
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
