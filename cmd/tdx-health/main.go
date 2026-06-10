package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/diagnostic"
	"github.com/quantbeing/tdx/model"
)

func main() {
	var hosts string
	var probe string
	var timeout time.Duration
	flag.StringVar(&hosts, "hosts", "", "comma-separated host[:port] list")
	flag.StringVar(&probe, "probe", "", "comma-separated diagnostic operation names, for example security-count,security-list-sh,quote")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "connect/read timeout")
	flag.Parse()

	servers := parseServers(hosts)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	transport := tdx.TransportOptions{
		ConnectTimeout: timeout,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
	}
	if strings.TrimSpace(probe) != "" {
		cmds, unknown := probeCommands(probe)
		if len(unknown) > 0 {
			_ = json.NewEncoder(os.Stdout).Encode(probeReport{OK: false, Error: "unknown probe operations", Unknown: unknown})
			os.Exit(2)
		}
		client, health, err := tdx.FromBestHostByOperations(ctx, tdx.Options{
			Servers:   servers,
			Timeout:   timeout,
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
		if encErr := json.NewEncoder(os.Stdout).Encode(report); encErr != nil {
			fmt.Fprintln(os.Stderr, encErr)
			os.Exit(1)
		}
		if err != nil {
			os.Exit(1)
		}
		return
	}

	results := tdx.PingAll(ctx, servers, transport)
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
