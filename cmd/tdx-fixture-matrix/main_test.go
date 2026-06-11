package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

type cliFakeCapturer struct{}

func (cliFakeCapturer) Capture(_ context.Context, cmd command.Command) (tdx.CapturedResponse, error) {
	return tdx.CapturedResponse{
		Operation:   cmd.Operation(),
		Server:      model.Server{Name: "fake", Host: "127.0.0.1", Port: 7709},
		Attempt:     1,
		Latency:     time.Millisecond,
		Request:     []byte{1},
		Header:      frame.Header{ZipSize: 1, UnzipSize: 1},
		HeaderBytes: make([]byte, frame.HeaderSize),
		RawBody:     []byte{2},
		Body:        []byte{2},
		Parsed:      uint16(2),
	}, nil
}

func TestRunRequiresLiveOptIn(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-out", t.TempDir()}, &out, cliFakeCapturer{}, func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "TDX_LIVE=1") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunWritesJSONLForSelectedOperations(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-out", t.TempDir(), "-ops", "security-count,quote"}, &out, cliFakeCapturer{}, func(string) string { return "1" })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", out.String())
	}
	if !strings.Contains(lines[0], `"name":"security-count"`) || !strings.Contains(lines[1], `"name":"quote"`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestRunRejectsUnknownOperation(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-out", t.TempDir(), "-ops", "missing"}, &out, cliFakeCapturer{}, func(string) string { return "1" })
	if err == nil || !strings.Contains(err.Error(), "unknown operations") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunRejectsUnknownRetryStrategy(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"-out", t.TempDir(), "-retry-strategy", "sticky"}, &out, cliFakeCapturer{}, func(string) string { return "1" })
	if err == nil || !strings.Contains(err.Error(), "unknown retry strategy") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildClientOptionsUsesRetryOptions(t *testing.T) {
	opts, err := buildClientOptions(12*time.Second, 3, "same-host", 2)
	if err != nil {
		t.Fatalf("buildClientOptions: %v", err)
	}
	if opts.Timeout != 12*time.Second {
		t.Fatalf("Timeout = %s, want 12s", opts.Timeout)
	}
	if opts.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts = %d, want 3", opts.MaxAttempts)
	}
	if opts.Retry != (tdx.RetryOptions{Strategy: tdx.RetryStrategySameHostFirst, SameHostAttempts: 2}) {
		t.Fatalf("Retry = %+v", opts.Retry)
	}
}
