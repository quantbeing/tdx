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
	"github.com/quantbeing/tdx/model"
)

func main() {
	var hosts string
	var timeout time.Duration
	flag.StringVar(&hosts, "hosts", "", "comma-separated host[:port] list")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "connect/read timeout")
	flag.Parse()

	servers := parseServers(hosts)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	results := tdx.PingAll(ctx, servers, tdx.TransportOptions{
		ConnectTimeout: timeout,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
	})
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
