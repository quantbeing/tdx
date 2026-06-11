package cmdflags

import (
	"fmt"
	"strings"

	tdx "github.com/quantbeing/tdx"
)

func ParseRetryStrategy(raw string) (tdx.RetryStrategy, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "failover-first", "failover_first", "failover":
		return tdx.RetryStrategyFailoverFirst, nil
	case "same-host-first", "same_host_first", "same-host":
		return tdx.RetryStrategySameHostFirst, nil
	default:
		return "", fmt.Errorf("unknown retry strategy %q", raw)
	}
}

func RetryOptions(raw string, sameHostAttempts int) (tdx.RetryOptions, error) {
	strategy, err := ParseRetryStrategy(raw)
	if err != nil {
		return tdx.RetryOptions{}, err
	}
	return tdx.RetryOptions{Strategy: strategy, SameHostAttempts: sameHostAttempts}, nil
}
