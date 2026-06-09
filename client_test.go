package tdx

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/model"
	"github.com/quantbeing/tdx/tdxtest"
)

type roundTripFunc func(context.Context, command.Command) (any, error)

func (f roundTripFunc) RoundTrip(ctx context.Context, cmd command.Command) (any, error) {
	return f(ctx, cmd)
}

func (f roundTripFunc) Close() error { return nil }

type closeAwareRoundTripper struct {
	fn      func(context.Context, command.Command) (any, error)
	onClose func()
	closed  atomic.Bool
}

func (r *closeAwareRoundTripper) RoundTrip(ctx context.Context, cmd command.Command) (any, error) {
	return r.fn(ctx, cmd)
}

func (r *closeAwareRoundTripper) Close() error {
	if r.closed.CompareAndSwap(false, true) && r.onClose != nil {
		r.onClose()
	}
	return nil
}

func TestClientObserverReceivesAttemptEvents(t *testing.T) {
	var events []RequestEvent
	client := NewClient(Options{
		Servers:     []model.Server{{Name: "bad", Host: "bad", Port: 7709}, {Name: "good", Host: "good", Port: 7709}},
		MaxAttempts: 2,
		Pool:        PoolOptions{Disable: true},
		Observer: ObserverFunc(func(event RequestEvent) {
			events = append(events, event)
		}),
		Dialer: DialerFunc(func(_ context.Context, server model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				if server.Name == "bad" {
					return nil, errors.New("read timeout")
				}
				return []model.Security{{Market: model.MarketSH, Code: "600519"}, {Market: model.MarketSH, Code: "600000"}}, nil
			}), nil
		}),
	})

	items, err := client.GetSecurityList(context.Background(), model.MarketSH, 0)
	if err != nil {
		t.Fatalf("GetSecurityList: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d", len(items))
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Operation != "security_list" || events[0].Server.Name != "bad" || events[0].Attempt != 1 || events[0].OK || events[0].Error == "" {
		t.Fatalf("first event = %+v", events[0])
	}
	if events[1].Operation != "security_list" || events[1].Server.Name != "good" || events[1].Attempt != 2 || !events[1].OK || events[1].Rows != 2 || events[1].Error != "" {
		t.Fatalf("second event = %+v", events[1])
	}
}

func TestMetricsCollectorAggregatesRequestEvents(t *testing.T) {
	metrics := NewMetricsCollector()
	client := NewClient(Options{
		Servers:  []model.Server{{Name: "good", Host: "good", Port: 7709}},
		Pool:     PoolOptions{Disable: true},
		Observer: metrics,
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				return []model.Security{{Market: model.MarketSH, Code: "600519"}, {Market: model.MarketSH, Code: "600000"}}, nil
			}), nil
		}),
	})

	for i := 0; i < 2; i++ {
		if _, err := client.GetSecurityList(context.Background(), model.MarketSH, 0); err != nil {
			t.Fatalf("GetSecurityList[%d]: %v", i, err)
		}
	}

	snapshots := metrics.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %+v", snapshots)
	}
	got := snapshots[0]
	if got.Operation != "security_list" || got.Server.Name != "good" || got.Attempts != 2 || got.Successes != 2 || got.Failures != 0 || got.TotalRows != 4 {
		t.Fatalf("snapshot = %+v", got)
	}
	if got.LastLatency <= 0 || got.MaxLatency <= 0 {
		t.Fatalf("latency snapshot = %+v", got)
	}
}

func TestClientReusesSuccessfulConnectionFromPool(t *testing.T) {
	var dials int32
	var closes int32
	client := NewClient(Options{
		Servers: []model.Server{{Name: "good", Host: "good", Port: 7709}},
		Pool:    PoolOptions{MaxIdlePerHost: 1},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			atomic.AddInt32(&dials, 1)
			return &closeAwareRoundTripper{
				fn: func(context.Context, command.Command) (any, error) {
					return uint16(100), nil
				},
				onClose: func() { atomic.AddInt32(&closes, 1) },
			}, nil
		}),
	})

	for i := 0; i < 2; i++ {
		count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
		if err != nil {
			t.Fatalf("GetSecurityCount[%d]: %v", i, err)
		}
		if count != 100 {
			t.Fatalf("count[%d] = %d", i, count)
		}
	}
	if dials != 1 {
		t.Fatalf("dials = %d, want 1", dials)
	}
	if closes != 0 {
		t.Fatalf("closes before client.Close = %d, want 0", closes)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closes != 1 {
		t.Fatalf("closes after client.Close = %d, want 1", closes)
	}
}

func TestClientDiscardsConnectionAfterRequestError(t *testing.T) {
	var dials int32
	var closes int32
	client := NewClient(Options{
		Servers:     []model.Server{{Name: "good", Host: "good", Port: 7709}},
		MaxAttempts: 1,
		Pool:        PoolOptions{MaxIdlePerHost: 1},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			dial := atomic.AddInt32(&dials, 1)
			return &closeAwareRoundTripper{
				fn: func(context.Context, command.Command) (any, error) {
					if dial == 1 {
						return nil, errors.New("read timeout")
					}
					return uint16(101), nil
				},
				onClose: func() { atomic.AddInt32(&closes, 1) },
			}, nil
		}),
	})

	if _, err := client.GetSecurityCount(context.Background(), model.MarketSH); err == nil {
		t.Fatal("first GetSecurityCount unexpectedly succeeded")
	}
	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("second GetSecurityCount: %v", err)
	}
	if count != 101 {
		t.Fatalf("count = %d, want 101", count)
	}
	if dials != 2 {
		t.Fatalf("dials = %d, want 2", dials)
	}
	if closes != 1 {
		t.Fatalf("closes after failed request = %d, want 1", closes)
	}
}

func TestClientFailsOverAfterFakeServerBadZlib(t *testing.T) {
	bad, err := tdxtest.StartScript(tdxtest.Script{
		Connections: []tdxtest.ConnectionScript{
			{Actions: append(tdxSetupActions(), tdxtest.ReadAndBadZlib([]byte{1, 2, 3}, 8))},
		},
	})
	if err != nil {
		t.Fatalf("start bad server: %v", err)
	}
	defer bad.Close()
	good, err := tdxtest.StartScript(tdxtest.Script{
		Connections: []tdxtest.ConnectionScript{
			{Actions: append(tdxSetupActions(), tdxtest.ReadAndRespond([]byte{0xd2, 0x04}))},
		},
	})
	if err != nil {
		t.Fatalf("start good server: %v", err)
	}
	defer good.Close()

	client := NewClient(Options{
		Servers:     []model.Server{serverFromAddr(t, "bad", bad.Addr), serverFromAddr(t, "good", good.Addr)},
		MaxAttempts: 2,
		Pool:        PoolOptions{Disable: true},
		Timeout:     time.Second,
	})

	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("GetSecurityCount: %v", err)
	}
	if count != 1234 {
		t.Fatalf("count = %d, want 1234", count)
	}
	stats := client.ServerStats()
	if stats[0].Failures != 1 || stats[1].Successes != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestClientFailsOverAfterFakeServerPartialFrame(t *testing.T) {
	bad, err := tdxtest.StartScript(tdxtest.Script{
		Connections: []tdxtest.ConnectionScript{
			{Actions: append(tdxSetupActions(), tdxtest.ReadAndPartialFrame([]byte{0, 0}, 4))},
		},
	})
	if err != nil {
		t.Fatalf("start bad server: %v", err)
	}
	defer bad.Close()
	good, err := tdxtest.StartScript(tdxtest.Script{
		Connections: []tdxtest.ConnectionScript{
			{Actions: append(tdxSetupActions(), tdxtest.ReadAndRespond([]byte{0x2a, 0x00}))},
		},
	})
	if err != nil {
		t.Fatalf("start good server: %v", err)
	}
	defer good.Close()

	client := NewClient(Options{
		Servers:     []model.Server{serverFromAddr(t, "bad", bad.Addr), serverFromAddr(t, "good", good.Addr)},
		MaxAttempts: 2,
		Pool:        PoolOptions{Disable: true},
		Timeout:     time.Second,
	})

	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("GetSecurityCount: %v", err)
	}
	if count != 42 {
		t.Fatalf("count = %d, want 42", count)
	}
}

func TestClientFailsOverByOperationAfterRequestError(t *testing.T) {
	var calls int32
	first := roundTripFunc(func(context.Context, command.Command) (any, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("read timeout")
	})
	second := roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
		atomic.AddInt32(&calls, 1)
		if cmd.Operation() != "security_count" {
			t.Fatalf("operation = %s, want security_count", cmd.Operation())
		}
		return uint16(27215), nil
	})
	dialers := []RoundTripper{first, second}
	var idx int32
	client := NewClient(Options{
		Servers:     []model.Server{{Host: "bad", Port: 7709}, {Host: "good", Port: 7709}},
		MaxAttempts: 2,
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			i := atomic.AddInt32(&idx, 1) - 1
			return dialers[i], nil
		}),
	})

	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("GetSecurityCount: %v", err)
	}
	if count != 27215 || calls != 2 {
		t.Fatalf("count=%d calls=%d, want 27215 and 2 calls", count, calls)
	}
	stats := client.ServerStats()
	if len(stats) != 2 || stats[0].Failures != 1 || stats[1].Successes != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestClientSkipsCoolingHostForSameOperation(t *testing.T) {
	var calls int32
	var hosts []string
	client := NewClient(Options{
		Servers: []model.Server{{Name: "bad", Host: "bad", Port: 7709}, {Name: "good", Host: "good", Port: 7709}},
		Pool:    PoolOptions{Disable: true},
		CircuitBreaker: CircuitBreakerOptions{
			FailureThreshold: 1,
			Cooldown:         time.Hour,
		},
		Dialer: DialerFunc(func(_ context.Context, server model.Server, _ TransportOptions) (RoundTripper, error) {
			hosts = append(hosts, server.Name)
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				if server.Name == "bad" {
					return nil, errors.New("operation timeout")
				}
				atomic.AddInt32(&calls, 1)
				return uint16(100), nil
			}), nil
		}),
	})

	count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("first GetSecurityCount: %v", err)
	}
	if count != 100 {
		t.Fatalf("count = %d", count)
	}
	count, err = client.GetSecurityCount(context.Background(), model.MarketSH)
	if err != nil {
		t.Fatalf("second GetSecurityCount: %v", err)
	}
	if count != 100 || atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("count=%d calls=%d", count, calls)
	}
	if len(hosts) != 3 || hosts[0] != "bad" || hosts[1] != "good" || hosts[2] != "good" {
		t.Fatalf("hosts = %v, want bad,good,good", hosts)
	}
	stats := client.OperationStats("security_count")
	if len(stats) != 2 || !stats[0].Cooling || stats[1].Cooling {
		t.Fatalf("operation stats = %+v", stats)
	}
}

func TestClientCooldownIsOperationAware(t *testing.T) {
	var hosts []string
	client := NewClient(Options{
		Servers:     []model.Server{{Name: "host-a", Host: "a", Port: 7709}, {Name: "host-b", Host: "b", Port: 7709}},
		MaxAttempts: 1,
		Pool:        PoolOptions{Disable: true},
		CircuitBreaker: CircuitBreakerOptions{
			FailureThreshold: 1,
			Cooldown:         time.Hour,
		},
		Dialer: DialerFunc(func(_ context.Context, server model.Server, _ TransportOptions) (RoundTripper, error) {
			hosts = append(hosts, server.Name)
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				if cmd.Operation() == "security_count" {
					return nil, errors.New("count unavailable")
				}
				return []model.Security{}, nil
			}), nil
		}),
	})

	if _, err := client.GetSecurityCount(context.Background(), model.MarketSH); err == nil {
		t.Fatal("GetSecurityCount unexpectedly succeeded")
	}
	if _, err := client.GetSecurityList(context.Background(), model.MarketSH, 0); err != nil {
		t.Fatalf("GetSecurityList: %v", err)
	}
	if len(hosts) != 2 || hosts[0] != "host-a" || hosts[1] != "host-b" {
		t.Fatalf("hosts = %v, want host-a,host-b", hosts)
	}
	countStats := client.OperationStats("security_count")
	listStats := client.OperationStats("security_list")
	if len(countStats) != 2 || !countStats[0].Cooling {
		t.Fatalf("count stats = %+v", countStats)
	}
	if len(listStats) != 2 || listStats[0].Cooling || listStats[1].Cooling {
		t.Fatalf("list stats = %+v", listStats)
	}
}

func TestKeepAliveClosesConnectionAfterFailures(t *testing.T) {
	var closed int32
	rt := &closeAwareRoundTripper{
		fn: func(context.Context, command.Command) (any, error) {
			return nil, net.ErrClosed
		},
		onClose: func() { atomic.AddInt32(&closed, 1) },
	}
	ka := NewKeepAliveManager(KeepAliveOptions{
		Interval:    10 * time.Millisecond,
		MaxFailures: 2,
		Command: func() command.Command {
			return command.NewSecurityCountCommand(model.MarketSH)
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ka.Start(ctx, rt)
	time.Sleep(60 * time.Millisecond)
	if atomic.LoadInt32(&closed) == 0 {
		t.Fatal("connection was not closed after repeated heartbeat failures")
	}
}

func TestClientGetSecurityQuotesSplitsBatchesAtProtocolLimit(t *testing.T) {
	var calls int32
	client := NewClient(Options{
		Servers: []model.Server{{Host: "good", Port: 7709}},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				calls++
				quoteCmd, ok := cmd.(command.SecurityQuotesCommand)
				if !ok {
					t.Fatalf("cmd = %T, want SecurityQuotesCommand", cmd)
				}
				if len(quoteCmd.Symbols) > 80 {
					t.Fatalf("batch size = %d, want <= 80", len(quoteCmd.Symbols))
				}
				out := make([]model.Quote, len(quoteCmd.Symbols))
				for i, sym := range quoteCmd.Symbols {
					out[i] = model.Quote{Market: sym.Market, Code: sym.Code}
				}
				return out, nil
			}), nil
		}),
	})
	symbols := make([]model.Symbol, 81)
	for i := range symbols {
		symbols[i] = model.Symbol{Market: model.MarketSH, Code: "600519"}
	}
	got, err := client.GetSecurityQuotes(context.Background(), symbols)
	if err != nil {
		t.Fatalf("GetSecurityQuotes: %v", err)
	}
	if len(got) != 81 || calls != 2 {
		t.Fatalf("len=%d calls=%d, want 81 and 2", len(got), calls)
	}
}

func TestClientGetReportFileFetchesUntilShortChunk(t *testing.T) {
	var calls int32
	client := NewClient(Options{
		Servers: []model.Server{{Host: "good", Port: 7709}},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				calls++
				if cmd.Operation() != "report_file" {
					t.Fatalf("operation = %s, want report_file", cmd.Operation())
				}
				if calls == 1 {
					return bytesOfLen(DefaultFileChunkSize), nil
				}
				return []byte{1, 2, 3}, nil
			}), nil
		}),
	})
	got, err := client.GetReportFile(context.Background(), "base_info.zip")
	if err != nil {
		t.Fatalf("GetReportFile: %v", err)
	}
	if len(got) != DefaultFileChunkSize+3 || calls != 2 {
		t.Fatalf("len=%d calls=%d", len(got), calls)
	}
}

func TestClientGetMarketStatFrom880005Quote(t *testing.T) {
	client := NewClient(Options{
		Servers: []model.Server{{Host: "good", Port: 7709}},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				if cmd.Operation() != "security_quotes" {
					t.Fatalf("operation = %s, want security_quotes", cmd.Operation())
				}
				return []model.Quote{{
					Code:     "880005",
					Price:    model.NewDecimal(5, 0),
					PreClose: model.NewDecimal(3, 0),
					Low:      model.NewDecimal(2, 0),
					High:     model.NewDecimal(11, 0),
					Amount:   123,
					Vol:      456,
				}}, nil
			}), nil
		}),
	})

	got, err := client.GetMarketStat(context.Background())
	if err != nil {
		t.Fatalf("GetMarketStat: %v", err)
	}
	if got.UpCount != 5 || got.DownCount != 3 || got.NeutralCount != 2 || got.SuspendedCount != 1 || got.TotalCount != 11 {
		t.Fatalf("stat = %+v", got)
	}
	if got.TotalAmount != 123 || got.TotalVolume != 456 {
		t.Fatalf("amount/volume = %f/%f", got.TotalAmount, got.TotalVolume)
	}
}

func TestClientGetFundFlowClassifiesTransactionAmounts(t *testing.T) {
	var calls int32
	client := NewClient(Options{
		Servers: []model.Server{{Host: "good", Port: 7709}},
		Dialer: DialerFunc(func(_ context.Context, _ model.Server, _ TransportOptions) (RoundTripper, error) {
			return roundTripFunc(func(_ context.Context, cmd command.Command) (any, error) {
				if cmd.Operation() != "transaction" {
					t.Fatalf("operation = %s, want transaction", cmd.Operation())
				}
				if atomic.AddInt32(&calls, 1) == 1 {
					return []model.Transaction{
						{Hour: 9, Minute: 31, Price: model.NewDecimal(1000, 2), Vol: 2000, BuyOrSell: 0},
						{Hour: 9, Minute: 32, Price: model.NewDecimal(1000, 2), Vol: 300, BuyOrSell: 1},
						{Hour: 9, Minute: 33, Price: model.NewDecimal(1000, 2), Vol: 10, BuyOrSell: 2},
					}, nil
				}
				return []model.Transaction{}, nil
			}), nil
		}),
	})

	got, err := client.GetFundFlow(context.Background(), model.MarketSH, "600519")
	if err != nil {
		t.Fatalf("GetFundFlow: %v", err)
	}
	if got.SuperIn != 2_000_000 || got.LargeOut != 300_000 || got.SmallIn != 0 || got.SmallOut != 0 {
		t.Fatalf("flow = %+v", got)
	}
	if got.MainNetInflow() != 1_700_000 || got.TotalNetInflow() != 1_700_000 {
		t.Fatalf("net = %f/%f", got.MainNetInflow(), got.TotalNetInflow())
	}
}

func TestClientCapturePreservesRawFrameAndParsedResponse(t *testing.T) {
	server, err := tdxtest.Start([][]byte{
		{}, {}, {},
		{0xd2, 0x04},
	})
	if err != nil {
		t.Fatalf("start fake server: %v", err)
	}
	defer server.Close()

	host, portRaw, err := net.SplitHostPort(server.Addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	client := NewClient(Options{
		Servers: []model.Server{{Name: "fake", Host: host, Port: port}},
		Timeout: time.Second,
	})

	got, err := client.Capture(context.Background(), command.NewSecurityCountCommand(model.MarketSH))
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got.Operation != "security_count" || got.Server.Name != "fake" {
		t.Fatalf("operation/server = %s/%+v", got.Operation, got.Server)
	}
	if got.Header.ZipSize != 2 || got.Header.UnzipSize != 2 {
		t.Fatalf("header = %+v", got.Header)
	}
	if !bytes.Equal(got.RawBody, []byte{0xd2, 0x04}) || !bytes.Equal(got.Body, []byte{0xd2, 0x04}) {
		t.Fatalf("raw/body = %x/%x", got.RawBody, got.Body)
	}
	if got.Parsed.(uint16) != 1234 {
		t.Fatalf("parsed = %#v", got.Parsed)
	}
	if len(got.Request) == 0 || len(got.HeaderBytes) != 16 || got.Latency <= 0 {
		t.Fatalf("capture missing request/header/latency: %+v", got)
	}
}

func bytesOfLen(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i)
	}
	return out
}

func tdxSetupActions() []tdxtest.Action {
	return []tdxtest.Action{
		tdxtest.ReadAndRespond(nil),
		tdxtest.ReadAndRespond(nil),
		tdxtest.ReadAndRespond(nil),
	}
}

func serverFromAddr(t *testing.T, name string, addr string) model.Server {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return model.Server{Name: name, Host: host, Port: port}
}
