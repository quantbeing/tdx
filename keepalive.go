package tdx

import (
	"context"
	"time"

	"github.com/quantbeing/tdx/command"
)

type KeepAliveOptions struct {
	Interval    time.Duration
	MaxFailures int
	Command     func() command.Command
}

type KeepAliveManager struct {
	opts KeepAliveOptions
}

func NewKeepAliveManager(opts KeepAliveOptions) *KeepAliveManager {
	if opts.Interval <= 0 {
		opts.Interval = 60 * time.Second
	}
	if opts.MaxFailures <= 0 {
		opts.MaxFailures = 3
	}
	return &KeepAliveManager{opts: opts}
}

func (m *KeepAliveManager) Start(ctx context.Context, rt RoundTripper) {
	go func() {
		ticker := time.NewTicker(m.opts.Interval)
		defer ticker.Stop()
		failures := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cmd := m.opts.Command
				if cmd == nil {
					return
				}
				if _, err := rt.RoundTrip(ctx, cmd()); err != nil {
					failures++
					if failures >= m.opts.MaxFailures {
						_ = rt.Close()
						return
					}
					continue
				}
				failures = 0
			}
		}
	}()
}
