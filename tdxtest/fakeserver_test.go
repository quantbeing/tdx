package tdxtest

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	tdx "github.com/quantbeing/tdx"
	"github.com/quantbeing/tdx/model"
)

func TestScriptedServerCanReturnBadZlibFrame(t *testing.T) {
	server, err := StartScript(Script{
		Connections: []ConnectionScript{
			{Actions: append(SetupResponses(), ReadAndBadZlib([]byte{1, 2, 3}, 8))},
		},
	})
	if err != nil {
		t.Fatalf("StartScript: %v", err)
	}
	defer server.Close()

	client := clientForServer(t, server)
	_, err = client.GetSecurityCount(context.Background(), model.MarketSH)
	if err == nil || !strings.Contains(err.Error(), "zlib") {
		t.Fatalf("err = %v, want zlib error", err)
	}
}

func TestScriptedServerCanCloseAfterPartialFrame(t *testing.T) {
	server, err := StartScript(Script{
		Connections: []ConnectionScript{
			{Actions: append(SetupResponses(), ReadAndPartialFrame([]byte{0, 0, 0}, 4))},
		},
	})
	if err != nil {
		t.Fatalf("StartScript: %v", err)
	}
	defer server.Close()

	client := clientForServer(t, server)
	_, err = client.GetSecurityCount(context.Background(), model.MarketSH)
	if err == nil {
		t.Fatal("GetSecurityCount unexpectedly succeeded")
	}
}

func TestScriptedServerCanDelayResponses(t *testing.T) {
	server, err := StartScript(Script{
		Connections: []ConnectionScript{
			{Actions: append(SetupResponses(), ReadAndDelay(20*time.Millisecond), ReadAndRespond([]byte{1, 0}))},
		},
	})
	if err != nil {
		t.Fatalf("StartScript: %v", err)
	}
	defer server.Close()

	start := time.Now()
	client := clientForServer(t, server)
	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("GetSecurityCount: %v", err)
	}
	if count != 1 || time.Since(start) < 20*time.Millisecond {
		t.Fatalf("count=%d elapsed=%s", count, time.Since(start))
	}
}

func clientForServer(t *testing.T, server *Server) *tdx.Client {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(server.Addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return tdx.NewClient(tdx.Options{
		Servers:     []model.Server{{Name: "fake", Host: host, Port: port}},
		MaxAttempts: 1,
		Pool:        tdx.PoolOptions{Disable: true},
		Timeout:     time.Second,
	})
}

func SetupResponses() []Action {
	return []Action{
		ReadAndRespond(nil),
		ReadAndRespond(nil),
		ReadAndRespond(nil),
	}
}
