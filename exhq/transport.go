package exhq

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
	"github.com/quantbeing/tdx/frame"
)

type NetDialer struct{}

func (NetDialer) DialExHQ(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error) {
	timeout := opts.ConnectTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", server.Addr())
	if err != nil {
		return nil, err
	}
	rt := &tcpRoundTripper{conn: conn, opts: opts}
	setupDeadline := time.Now().Add(timeout)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(setupDeadline) {
		setupDeadline = deadline
	}
	_ = conn.SetDeadline(setupDeadline)
	if err := rt.setup(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return rt, nil
}

type tcpRoundTripper struct {
	conn net.Conn
	opts TransportOptions
	mu   sync.Mutex
}

func (t *tcpRoundTripper) RoundTrip(ctx context.Context, cmd command.Command) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := cmd.BuildRequest()
	if err != nil {
		return nil, err
	}

	stopCancelWatch := t.interruptOnCancel(ctx)
	defer stopCancelWatch()

	if err := t.conn.SetWriteDeadline(operationDeadline(ctx, t.opts.WriteTimeout)); err != nil {
		return nil, errorWithContext(ctx, err)
	}
	if _, err := t.conn.Write(req); err != nil {
		return nil, errorWithContext(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := t.conn.SetReadDeadline(operationDeadline(ctx, t.opts.ReadTimeout)); err != nil {
		return nil, errorWithContext(ctx, err)
	}
	body, err := readFrame(t.conn)
	if err != nil {
		return nil, errorWithContext(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out, err := cmd.ParseResponse(body)
	if err != nil {
		return nil, errorWithContext(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (t *tcpRoundTripper) setup() error {
	for _, req := range command.ExSetupCommands {
		if _, err := t.conn.Write(req); err != nil {
			return err
		}
		if _, err := readFrame(t.conn); err != nil {
			return err
		}
	}
	return nil
}

func (t *tcpRoundTripper) Close() error {
	return t.conn.Close()
}

func readFrame(r io.Reader) ([]byte, error) {
	headerBytes := make([]byte, frame.HeaderSize)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, err
	}
	header, err := frame.ParseHeader(headerBytes)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, header.ZipSize)
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, err
	}
	return frame.DecodeBody(header, raw)
}

func (t *tcpRoundTripper) interruptOnCancel(ctx context.Context) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = t.conn.SetDeadline(time.Now().Add(-time.Second))
		close(done)
	})
	return func() {
		if !stop() {
			<-done
		}
	}
}

func operationDeadline(ctx context.Context, timeout time.Duration) time.Time {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
		deadline = ctxDeadline
	}
	return deadline
}

func errorWithContext(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		if err == nil {
			return ctxErr
		}
		return errors.Join(ctxErr, err)
	}
	return err
}
