package exhq_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quantbeing/tdx/exhq"
	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
	"github.com/quantbeing/tdx/frame"
)

func TestClientGetMarkets(t *testing.T) {
	marketBody := make([]byte, 2+64)
	binary.LittleEndian.PutUint16(marketBody[0:2], 1)
	row := marketBody[2:]
	row[0] = 1
	copy(row[1:33], []byte("Futures\x00"))
	row[33] = byte(model.MarketID(47))
	copy(row[34:36], []byte("IF"))
	row[62] = 0xaa
	row[63] = 0xbb

	srv := startExHQScript(t, func(conn net.Conn) {
		setup := readRequest(t, conn, len(command.ExSetupCommands[0]))
		if !bytes.Equal(setup, command.ExSetupCommands[0]) {
			t.Fatalf("setup request = %x", setup)
		}
		writeFrame(t, conn, nil)

		want, err := command.NewMarketsCommand().BuildRequest()
		if err != nil {
			t.Fatalf("markets request: %v", err)
		}
		req := readRequest(t, conn, len(want))
		if !bytes.Equal(req, want) {
			t.Fatalf("markets request = %x want %x", req, want)
		}
		writeFrame(t, conn, marketBody)
	})
	defer srv.Close()

	host, port := splitAddr(t, srv.Addr)
	c := exhq.NewClient(exhq.Options{
		Servers: []model.Server{{Host: host, Port: port}},
		Timeout: time.Second,
	})
	defer c.Close()

	got, err := c.GetMarkets(context.Background())
	if err != nil {
		t.Fatalf("GetMarkets: %v", err)
	}
	if len(got) != 1 || got[0].MarketID != model.MarketID(47) || got[0].Name != "Futures" || !bytes.Equal(got[0].Unknown, []byte{0xaa, 0xbb}) {
		t.Fatalf("markets = %+v", got)
	}
}

func TestClientGetMarketsCancelInterruptsBlockedRead(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseServer := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}

	srv := startExHQScript(t, func(conn net.Conn) {
		setup := readRequest(t, conn, len(command.ExSetupCommands[0]))
		if !bytes.Equal(setup, command.ExSetupCommands[0]) {
			t.Fatalf("setup request = %x", setup)
		}
		writeFrame(t, conn, nil)

		want, err := command.NewMarketsCommand().BuildRequest()
		if err != nil {
			t.Fatalf("markets request: %v", err)
		}
		req := readRequest(t, conn, len(want))
		if !bytes.Equal(req, want) {
			t.Fatalf("markets request = %x want %x", req, want)
		}

		<-release
	})
	defer srv.Close()
	defer releaseServer()

	host, port := splitAddr(t, srv.Addr)
	c := exhq.NewClient(exhq.Options{
		Servers: []model.Server{{Host: host, Port: port}},
		Timeout: 5 * time.Second,
	})
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := c.GetMarkets(ctx)
		done <- err
	}()
	time.AfterFunc(25*time.Millisecond, cancel)

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetMarkets error = %v, want context canceled", err)
		}
	case <-time.After(300 * time.Millisecond):
		releaseServer()
		select {
		case err := <-done:
			t.Fatalf("GetMarkets waited for server close after context cancel; err = %v", err)
		case <-time.After(time.Second):
			t.Fatal("GetMarkets stayed blocked after context cancel and server close")
		}
	}
}

func startExHQScript(t *testing.T, script func(net.Conn)) *testServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &testServer{Addr: ln.Addr().String(), ln: ln, done: make(chan struct{})}
	go func() {
		defer close(srv.done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		script(conn)
	}()
	return srv
}

type testServer struct {
	Addr string
	ln   net.Listener
	done chan struct{}
}

func (s *testServer) Close() {
	_ = s.ln.Close()
	<-s.done
}

func readRequest(t *testing.T, conn net.Conn, wantLen int) []byte {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	buf := make([]byte, wantLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read request: %v", err)
	}
	return buf
}

func writeFrame(t *testing.T, conn net.Conn, body []byte) {
	t.Helper()
	header := make([]byte, frame.HeaderSize)
	binary.LittleEndian.PutUint16(header[12:14], uint16(len(body)))
	binary.LittleEndian.PutUint16(header[14:16], uint16(len(body)))
	if _, err := conn.Write(append(header, body...)); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		t.Fatalf("addr has no port: %q", addr)
	}
	port, err := strconv.Atoi(addr[idx+1:])
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	return addr[:idx], port
}
