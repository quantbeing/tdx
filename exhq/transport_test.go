package exhq

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type roundTripTestCommand struct {
	req []byte
}

func (c roundTripTestCommand) BuildRequest() ([]byte, error) {
	return c.req, nil
}

func (c roundTripTestCommand) ParseResponse(body []byte) (any, error) {
	return body, nil
}

func (c roundTripTestCommand) Operation() string {
	return "round_trip_test"
}

func TestTCPRoundTripperContextCancelInterruptsBlockedWrite(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	rt := &tcpRoundTripper{
		conn: client,
		opts: TransportOptions{
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := rt.RoundTrip(ctx, roundTripTestCommand{req: []byte("blocked write")})
		done <- err
	}()
	time.AfterFunc(25*time.Millisecond, cancel)

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RoundTrip error = %v, want context canceled", err)
		}
	case <-time.After(300 * time.Millisecond):
		_ = server.Close()
		_ = rt.Close()
		err := waitRoundTripError(t, done)
		t.Fatalf("RoundTrip stayed blocked after context cancel; eventual error = %v", err)
	}
}

func TestTCPRoundTripperWriteTimeoutInterruptsBlockedWrite(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	rt := &tcpRoundTripper{
		conn: client,
		opts: TransportOptions{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 25 * time.Millisecond,
		},
	}
	defer rt.Close()

	done := make(chan error, 1)
	go func() {
		_, err := rt.RoundTrip(context.Background(), roundTripTestCommand{req: []byte("blocked write")})
		done <- err
	}()

	select {
	case err := <-done:
		if !isTimeoutError(err) {
			t.Fatalf("RoundTrip error = %v, want timeout", err)
		}
	case <-time.After(300 * time.Millisecond):
		_ = server.Close()
		_ = rt.Close()
		err := waitRoundTripError(t, done)
		t.Fatalf("RoundTrip did not honor WriteTimeout; eventual error = %v", err)
	}
}

func TestTCPRoundTripperReadTimeoutInterruptsBlockedRead(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	req := []byte("read timeout")
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, len(req))
		_, err := io.ReadFull(server, buf)
		readDone <- err
	}()

	rt := &tcpRoundTripper{
		conn: client,
		opts: TransportOptions{
			ReadTimeout:  25 * time.Millisecond,
			WriteTimeout: time.Second,
		},
	}
	defer rt.Close()

	done := make(chan error, 1)
	go func() {
		_, err := rt.RoundTrip(context.Background(), roundTripTestCommand{req: req})
		done <- err
	}()

	if err := waitRoundTripError(t, readDone); err != nil {
		t.Fatalf("server read request: %v", err)
	}
	err := waitRoundTripError(t, done)
	if !isTimeoutError(err) {
		t.Fatalf("RoundTrip error = %v, want timeout", err)
	}
}

func TestTCPRoundTripperCloseInterruptsBlockedRead(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	req := []byte("close read")
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, len(req))
		_, err := io.ReadFull(server, buf)
		readDone <- err
	}()

	rt := &tcpRoundTripper{
		conn: client,
		opts: TransportOptions{
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
	}

	done := make(chan error, 1)
	go func() {
		_, err := rt.RoundTrip(context.Background(), roundTripTestCommand{req: req})
		done <- err
	}()

	if err := waitRoundTripError(t, readDone); err != nil {
		t.Fatalf("server read request: %v", err)
	}
	time.AfterFunc(25*time.Millisecond, func() {
		_ = rt.Close()
	})

	err := waitRoundTripError(t, done)
	if err == nil {
		t.Fatal("RoundTrip error = nil, want close error")
	}
}

func waitRoundTripError(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for RoundTrip")
	}
	return nil
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
