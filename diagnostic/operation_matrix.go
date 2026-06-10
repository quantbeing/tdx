package diagnostic

import (
	"context"
	"sort"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
)

type OperationMatrixClient interface {
	HealthCheck(context.Context, ...command.Command) []tdx.OperationHealth
	Close() error
}

type OperationMatrixClientFactory func(model.Server, tdx.Observer) OperationMatrixClient

type OperationMatrixOptions struct {
	Servers               []model.Server
	Operations            []MatrixOperation
	Repeats               int
	PerOperationTimeout   time.Duration
	TimeoutRecommendation TimeoutRecommendationOptions
	NewClient             OperationMatrixClientFactory
}

type OperationMatrixAttempt struct {
	Attempt   int          `json:"attempt"`
	Server    model.Server `json:"server"`
	OK        bool         `json:"ok"`
	Error     string       `json:"error,omitempty"`
	LatencyMS int64        `json:"latency_ms"`
	Rows      int          `json:"rows,omitempty"`
	BodySize  int          `json:"body_size,omitempty"`
	Reused    bool         `json:"reused"`
}

type OperationMatrixResult struct {
	Run       int                      `json:"run"`
	Name      string                   `json:"name"`
	Operation string                   `json:"operation"`
	Server    model.Server             `json:"server"`
	OK        bool                     `json:"ok"`
	Error     string                   `json:"error,omitempty"`
	LatencyMS int64                    `json:"latency_ms"`
	Attempts  []OperationMatrixAttempt `json:"attempts,omitempty"`
}

type OperationMatrixSummary struct {
	Name         string       `json:"name"`
	Operation    string       `json:"operation"`
	Server       model.Server `json:"server"`
	Runs         int          `json:"runs"`
	Successes    int          `json:"successes"`
	Failures     int          `json:"failures"`
	SuccessRate  float64      `json:"success_rate"`
	MinLatencyMS int64        `json:"min_latency_ms"`
	AvgLatencyMS int64        `json:"avg_latency_ms"`
	MaxLatencyMS int64        `json:"max_latency_ms"`
	TotalRows    int          `json:"total_rows"`
	TotalBody    int          `json:"total_body_size"`
	LastError    string       `json:"last_error,omitempty"`
}

type OperationMatrixReport struct {
	StartedAt              time.Time                `json:"started_at"`
	FinishedAt             time.Time                `json:"finished_at"`
	DurationMS             int64                    `json:"duration_ms"`
	Results                []OperationMatrixResult  `json:"results"`
	Summary                []OperationMatrixSummary `json:"summary"`
	TimeoutRecommendations []TimeoutRecommendation  `json:"timeout_recommendations,omitempty"`
}

type TimeoutRecommendationOptions struct {
	MinTimeout         time.Duration
	MaxTimeout         time.Duration
	SuccessMultiplier  float64
	FailureFastTimeout time.Duration
}

type TimeoutRecommendation struct {
	Name                 string       `json:"name"`
	Operation            string       `json:"operation"`
	Server               model.Server `json:"server"`
	Runs                 int          `json:"runs"`
	Successes            int          `json:"successes"`
	Failures             int          `json:"failures"`
	MaxObservedLatencyMS int64        `json:"max_observed_latency_ms"`
	RecommendedTimeoutMS int64        `json:"recommended_timeout_ms"`
	Reason               string       `json:"reason"`
}

type operationMatrixClientError struct {
	err error
}

func (e operationMatrixClientError) HealthCheck(context.Context, ...command.Command) []tdx.OperationHealth {
	return []tdx.OperationHealth{{OK: false, Error: e.err.Error()}}
}

func (e operationMatrixClientError) Close() error {
	return nil
}

func RunOperationMatrix(ctx context.Context, opts OperationMatrixOptions) OperationMatrixReport {
	opts = normalizeOperationMatrixOptions(opts)
	report := OperationMatrixReport{StartedAt: time.Now()}
	for run := 1; run <= opts.Repeats; run++ {
		for _, server := range opts.Servers {
			for _, op := range opts.Operations {
				result := runOneOperationMatrixProbe(ctx, opts, run, server, op)
				report.Results = append(report.Results, result)
			}
		}
	}
	report.FinishedAt = time.Now()
	report.DurationMS = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	report.Summary = summarizeOperationMatrix(report.Results)
	report.TimeoutRecommendations = RecommendOperationTimeouts(report.Summary, opts.TimeoutRecommendation)
	return report
}

func normalizeOperationMatrixOptions(opts OperationMatrixOptions) OperationMatrixOptions {
	if opts.Repeats <= 0 {
		opts.Repeats = 1
	}
	if opts.PerOperationTimeout <= 0 {
		opts.PerOperationTimeout = 8 * time.Second
	}
	if len(opts.Operations) == 0 {
		opts.Operations = []MatrixOperation{{Name: "security-count", Command: command.NewSecurityCountCommand(model.MarketSH)}}
	}
	if opts.NewClient == nil {
		opts.NewClient = func(server model.Server, observer tdx.Observer) OperationMatrixClient {
			return tdx.NewClient(tdx.Options{
				Servers:     []model.Server{server},
				MaxAttempts: 1,
				Timeout:     opts.PerOperationTimeout,
				Transport: tdx.TransportOptions{
					ConnectTimeout: opts.PerOperationTimeout,
					ReadTimeout:    opts.PerOperationTimeout,
					WriteTimeout:   opts.PerOperationTimeout,
				},
				Pool:     tdx.PoolOptions{MaxIdlePerHost: 0},
				Observer: observer,
			})
		}
	}
	return opts
}

func runOneOperationMatrixProbe(ctx context.Context, opts OperationMatrixOptions, run int, server model.Server, op MatrixOperation) OperationMatrixResult {
	collector := &operationMatrixAttemptCollector{}
	client := opts.NewClient(server, collector)
	defer func() {
		if client != nil {
			_ = client.Close()
		}
	}()

	result := OperationMatrixResult{
		Run:       run,
		Name:      op.Name,
		Operation: op.Command.Operation(),
		Server:    server,
	}
	opCtx, cancel := context.WithTimeout(ctx, opts.PerOperationTimeout)
	defer cancel()
	start := time.Now()
	health := client.HealthCheck(opCtx, op.Command)
	result.LatencyMS = time.Since(start).Milliseconds()
	result.Attempts = collector.attempts()
	if len(health) == 0 {
		result.Error = "no health result"
		return result
	}
	result.OK = health[0].OK
	if health[0].Latency > 0 {
		result.LatencyMS = health[0].Latency.Milliseconds()
	}
	result.Error = health[0].Error
	return result
}

type operationMatrixAttemptCollector struct {
	events []OperationMatrixAttempt
}

func (c *operationMatrixAttemptCollector) OnRequest(event tdx.RequestEvent) {
	c.events = append(c.events, OperationMatrixAttempt{
		Attempt:   event.Attempt,
		Server:    event.Server,
		OK:        event.OK,
		Error:     event.Error,
		LatencyMS: event.Latency.Milliseconds(),
		Rows:      event.Rows,
		BodySize:  event.BodySize,
		Reused:    event.Reused,
	})
}

func (c *operationMatrixAttemptCollector) attempts() []OperationMatrixAttempt {
	return append([]OperationMatrixAttempt(nil), c.events...)
}

func summarizeOperationMatrix(results []OperationMatrixResult) []OperationMatrixSummary {
	byKey := make(map[string]*OperationMatrixSummary)
	for _, result := range results {
		key := result.Operation + "\x00" + result.Server.Addr()
		summary, ok := byKey[key]
		if !ok {
			summary = &OperationMatrixSummary{
				Name:         result.Name,
				Operation:    result.Operation,
				Server:       result.Server,
				MinLatencyMS: result.LatencyMS,
			}
			byKey[key] = summary
		}
		summary.Runs++
		if result.OK {
			summary.Successes++
		} else {
			summary.Failures++
			summary.LastError = result.Error
		}
		if result.LatencyMS < summary.MinLatencyMS {
			summary.MinLatencyMS = result.LatencyMS
		}
		if result.LatencyMS > summary.MaxLatencyMS {
			summary.MaxLatencyMS = result.LatencyMS
		}
		summary.AvgLatencyMS += result.LatencyMS
		for _, attempt := range result.Attempts {
			summary.TotalRows += attempt.Rows
			summary.TotalBody += attempt.BodySize
		}
	}
	out := make([]OperationMatrixSummary, 0, len(byKey))
	for _, summary := range byKey {
		if summary.Runs > 0 {
			summary.SuccessRate = float64(summary.Successes) / float64(summary.Runs)
			summary.AvgLatencyMS = summary.AvgLatencyMS / int64(summary.Runs)
		}
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Operation != out[j].Operation {
			return out[i].Operation < out[j].Operation
		}
		return out[i].Server.Addr() < out[j].Server.Addr()
	})
	return out
}

func RecommendOperationTimeouts(summaries []OperationMatrixSummary, opts TimeoutRecommendationOptions) []TimeoutRecommendation {
	opts = normalizeTimeoutRecommendationOptions(opts)
	out := make([]TimeoutRecommendation, 0, len(summaries))
	for _, summary := range summaries {
		timeout := opts.FailureFastTimeout
		reason := "no_success_fail_fast"
		if summary.Successes > 0 {
			timeout = time.Duration(float64(time.Duration(summary.MaxLatencyMS)*time.Millisecond) * opts.SuccessMultiplier)
			timeout = clampDuration(timeout, opts.MinTimeout, opts.MaxTimeout)
			reason = "observed_success_latency"
		} else {
			if summary.MaxLatencyMS > 0 {
				observed := time.Duration(summary.MaxLatencyMS) * time.Millisecond
				if observed < timeout {
					timeout = observed
				}
			}
			timeout = clampDuration(timeout, opts.MinTimeout, opts.MaxTimeout)
		}
		out = append(out, TimeoutRecommendation{
			Name:                 summary.Name,
			Operation:            summary.Operation,
			Server:               summary.Server,
			Runs:                 summary.Runs,
			Successes:            summary.Successes,
			Failures:             summary.Failures,
			MaxObservedLatencyMS: summary.MaxLatencyMS,
			RecommendedTimeoutMS: timeout.Milliseconds(),
			Reason:               reason,
		})
	}
	return out
}

func normalizeTimeoutRecommendationOptions(opts TimeoutRecommendationOptions) TimeoutRecommendationOptions {
	if opts.MinTimeout <= 0 {
		opts.MinTimeout = 500 * time.Millisecond
	}
	if opts.MaxTimeout <= 0 {
		opts.MaxTimeout = 3 * time.Second
	}
	if opts.SuccessMultiplier <= 0 {
		opts.SuccessMultiplier = 4
	}
	if opts.FailureFastTimeout <= 0 {
		opts.FailureFastTimeout = 1500 * time.Millisecond
	}
	return opts
}

func clampDuration(value time.Duration, min time.Duration, max time.Duration) time.Duration {
	if min > 0 && value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}
