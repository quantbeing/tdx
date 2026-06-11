package cmdflags

import (
	"strings"
	"testing"

	tdx "github.com/quantbeing/tdx"
)

func TestParseRetryStrategyAliases(t *testing.T) {
	tests := map[string]tdx.RetryStrategy{
		"failover-first":  tdx.RetryStrategyFailoverFirst,
		"failover_first":  tdx.RetryStrategyFailoverFirst,
		"failover":        tdx.RetryStrategyFailoverFirst,
		"same-host-first": tdx.RetryStrategySameHostFirst,
		"same_host_first": tdx.RetryStrategySameHostFirst,
		"same-host":       tdx.RetryStrategySameHostFirst,
	}
	for raw, want := range tests {
		got, err := ParseRetryStrategy(raw)
		if err != nil {
			t.Fatalf("ParseRetryStrategy(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("ParseRetryStrategy(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseRetryStrategyRejectsUnknown(t *testing.T) {
	_, err := ParseRetryStrategy("sticky")
	if err == nil || !strings.Contains(err.Error(), "unknown retry strategy") {
		t.Fatalf("err = %v", err)
	}
}

func TestRetryOptions(t *testing.T) {
	got, err := RetryOptions("same-host", 2)
	if err != nil {
		t.Fatalf("RetryOptions: %v", err)
	}
	want := tdx.RetryOptions{Strategy: tdx.RetryStrategySameHostFirst, SameHostAttempts: 2}
	if got != want {
		t.Fatalf("RetryOptions = %+v, want %+v", got, want)
	}
}
