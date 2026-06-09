package tdx

import (
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/quantbeing/tdx/model"
)

type Observer interface {
	OnRequest(RequestEvent)
}

type ObserverFunc func(RequestEvent)

func (f ObserverFunc) OnRequest(event RequestEvent) {
	f(event)
}

type RequestEvent struct {
	Operation string        `json:"operation"`
	Server    model.Server  `json:"server"`
	Attempt   int           `json:"attempt"`
	OK        bool          `json:"ok"`
	Error     string        `json:"error,omitempty"`
	Latency   time.Duration `json:"latency"`
	Rows      int           `json:"rows,omitempty"`
	BodySize  int           `json:"body_size,omitempty"`
	Reused    bool          `json:"reused"`
}

type MetricsCollector struct {
	mu    sync.Mutex
	stats map[metricsKey]MetricsSnapshot
}

type metricsKey struct {
	operation string
	host      string
}

type MetricsSnapshot struct {
	Operation    string        `json:"operation"`
	Server       model.Server  `json:"server"`
	Attempts     uint64        `json:"attempts"`
	Successes    uint64        `json:"successes"`
	Failures     uint64        `json:"failures"`
	TotalRows    uint64        `json:"total_rows"`
	LastLatency  time.Duration `json:"last_latency"`
	MaxLatency   time.Duration `json:"max_latency"`
	TotalLatency time.Duration `json:"total_latency"`
	LastError    string        `json:"last_error,omitempty"`
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{stats: make(map[metricsKey]MetricsSnapshot)}
}

func (m *MetricsCollector) OnRequest(event RequestEvent) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stats == nil {
		m.stats = make(map[metricsKey]MetricsSnapshot)
	}
	key := metricsKey{operation: event.Operation, host: event.Server.Addr()}
	stat := m.stats[key]
	stat.Operation = event.Operation
	stat.Server = event.Server
	stat.Attempts++
	stat.TotalRows += uint64(event.Rows)
	stat.LastLatency = event.Latency
	stat.TotalLatency += event.Latency
	if event.Latency > stat.MaxLatency {
		stat.MaxLatency = event.Latency
	}
	if event.OK {
		stat.Successes++
		stat.LastError = ""
	} else {
		stat.Failures++
		stat.LastError = event.Error
	}
	m.stats[key] = stat
}

func (m *MetricsCollector) Snapshot() []MetricsSnapshot {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MetricsSnapshot, 0, len(m.stats))
	for _, stat := range m.stats {
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Operation != out[j].Operation {
			return out[i].Operation < out[j].Operation
		}
		return out[i].Server.Addr() < out[j].Server.Addr()
	})
	return out
}

func rowCount(value any) int {
	if value == nil {
		return 0
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return v.Len()
	default:
		return 0
	}
}
