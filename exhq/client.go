package exhq

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/quantbeing/tdx/exhq/command"
	"github.com/quantbeing/tdx/exhq/model"
)

type Options struct {
	Servers     []model.Server
	Timeout     time.Duration
	MaxAttempts int
	Dialer      Dialer
	Transport   TransportOptions
}

type TransportOptions struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

type Dialer interface {
	DialExHQ(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error)
}

type DialerFunc func(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error)

func (f DialerFunc) DialExHQ(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error) {
	return f(ctx, server, opts)
}

type RoundTripper interface {
	RoundTrip(ctx context.Context, cmd command.Command) (any, error)
	Close() error
}

type Client struct {
	servers  []model.Server
	opts     Options
	dialer   Dialer
	attempts int
}

type ListOptions struct {
	Start       int
	Count       int
	PageSize    int
	MaxPages    int
	StopOnError bool
}

func NewClient(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.Transport.ConnectTimeout <= 0 {
		opts.Transport.ConnectTimeout = opts.Timeout
	}
	if opts.Transport.ReadTimeout <= 0 {
		opts.Transport.ReadTimeout = opts.Timeout
	}
	if opts.Transport.WriteTimeout <= 0 {
		opts.Transport.WriteTimeout = opts.Timeout
	}
	if opts.Dialer == nil {
		opts.Dialer = NetDialer{}
	}
	if len(opts.Servers) == 0 {
		opts.Servers = KnownServers()
	}
	attempts := opts.MaxAttempts
	if attempts <= 0 || attempts > len(opts.Servers) {
		attempts = len(opts.Servers)
	}
	if attempts <= 0 {
		attempts = 1
	}
	return &Client{
		servers:  append([]model.Server(nil), opts.Servers...),
		opts:     opts,
		dialer:   opts.Dialer,
		attempts: attempts,
	}
}

func KnownServers() []model.Server {
	return []model.Server{
		{Name: "exhq-gz-210", Host: "121.14.110.210", Port: 7727},
		{Name: "exhq-sh-141", Host: "61.152.107.141", Port: 7727},
		{Name: "exhq-wh-130", Host: "119.97.142.130", Port: 7727},
	}
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) GetMarkets(ctx context.Context) ([]model.Market, error) {
	got, err := c.execute(ctx, command.NewMarketsCommand())
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Market)
	if !ok {
		return nil, fmt.Errorf("tdx exhq markets unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetInstrumentCount(ctx context.Context) (int, error) {
	got, err := c.execute(ctx, command.NewInstrumentCountCommand())
	if err != nil {
		return 0, err
	}
	count, ok := got.(int)
	if !ok {
		return 0, fmt.Errorf("tdx exhq instrument_count unexpected reply %T", got)
	}
	return count, nil
}

func (c *Client) GetInstrumentInfo(ctx context.Context, start, count int) ([]model.Instrument, error) {
	got, err := c.execute(ctx, command.NewInstrumentInfoCommand(start, count))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Instrument)
	if !ok {
		return nil, fmt.Errorf("tdx exhq instrument_info unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) ListInstruments(ctx context.Context, opts ListOptions) (model.PartialResult[model.Instrument], error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	start := opts.Start
	total := opts.Count
	if total <= 0 {
		count, err := c.GetInstrumentCount(ctx)
		if err != nil {
			return model.PartialResult[model.Instrument]{Failures: []model.OperationError{{Operation: "exhq_instrument_count", Err: err.Error()}}}, err
		}
		total = count - start
	}
	var result model.PartialResult[model.Instrument]
	pages := 0
	for fetched := 0; fetched < total; {
		if err := contextError(ctx); err != nil {
			return result, err
		}
		if opts.MaxPages > 0 && pages >= opts.MaxPages {
			result.Failures = append(result.Failures, model.OperationError{
				Operation: "exhq_instrument_info_budget",
				Start:     start + fetched,
				Count:     pageSize,
				Err:       fmt.Sprintf("max pages reached: %d", opts.MaxPages),
			})
			return result, fmt.Errorf("tdx exhq partial result: failures=%d", len(result.Failures))
		}
		count := pageSize
		if remain := total - fetched; remain < count {
			count = remain
		}
		rows, err := c.GetInstrumentInfo(ctx, start+fetched, count)
		if err != nil {
			result.Failures = append(result.Failures, model.OperationError{Operation: "exhq_instrument_info", Start: start + fetched, Count: count, Err: err.Error()})
			if opts.StopOnError {
				return result, fmt.Errorf("tdx exhq partial result: failures=%d", len(result.Failures))
			}
			break
		}
		pages++
		if len(rows) == 0 {
			break
		}
		result.Items = append(result.Items, rows...)
		fetched += len(rows)
		if len(rows) < count {
			break
		}
	}
	if len(result.Failures) > 0 {
		return result, fmt.Errorf("tdx exhq partial result: failures=%d", len(result.Failures))
	}
	return result, nil
}

func (c *Client) GetInstrumentQuote(ctx context.Context, market model.MarketID, code string) (model.Quote, error) {
	got, err := c.execute(ctx, command.NewInstrumentQuoteCommand(market, code))
	if err != nil {
		return model.Quote{}, err
	}
	quote, ok := got.(model.Quote)
	if !ok {
		return model.Quote{}, fmt.Errorf("tdx exhq instrument_quote unexpected reply %T", got)
	}
	return quote, nil
}

func (c *Client) GetInstrumentQuoteList(ctx context.Context, market model.MarketID, category uint8, start, count int) ([]model.Quote, error) {
	got, err := c.execute(ctx, command.NewInstrumentQuoteListCommand(market, category, start, count))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Quote)
	if !ok {
		return nil, fmt.Errorf("tdx exhq instrument_quote_list unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetInstrumentBars(ctx context.Context, market model.MarketID, code string, category model.KlineCategory, start, count int) ([]model.Bar, error) {
	got, err := c.execute(ctx, command.NewInstrumentBarsCommand(category, market, code, start, count))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Bar)
	if !ok {
		return nil, fmt.Errorf("tdx exhq instrument_bars unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetHistoryInstrumentBarsRange(ctx context.Context, market model.MarketID, code string, start, end int) ([]model.Bar, error) {
	got, err := c.execute(ctx, command.NewHistoryInstrumentBarsRangeCommand(market, code, start, end))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Bar)
	if !ok {
		return nil, fmt.Errorf("tdx exhq history_instrument_bars_range unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetMinuteTimeData(ctx context.Context, market model.MarketID, code string) ([]model.MinuteTime, error) {
	got, err := c.execute(ctx, command.NewMinuteTimeDataCommand(market, code))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.MinuteTime)
	if !ok {
		return nil, fmt.Errorf("tdx exhq minute_time unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetHistoryMinuteTimeData(ctx context.Context, market model.MarketID, code string, date int) ([]model.MinuteTime, error) {
	got, err := c.execute(ctx, command.NewHistoryMinuteTimeDataCommand(market, code, date))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.MinuteTime)
	if !ok {
		return nil, fmt.Errorf("tdx exhq history_minute_time unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetTransactionData(ctx context.Context, market model.MarketID, code string, start, count int) ([]model.Transaction, error) {
	got, err := c.execute(ctx, command.NewTransactionDataCommand(market, code, start, count))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Transaction)
	if !ok {
		return nil, fmt.Errorf("tdx exhq transaction unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) GetHistoryTransactionData(ctx context.Context, market model.MarketID, code string, date, start, count int) ([]model.Transaction, error) {
	got, err := c.execute(ctx, command.NewHistoryTransactionDataCommand(market, code, date, start, count))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.Transaction)
	if !ok {
		return nil, fmt.Errorf("tdx exhq history_transaction unexpected reply %T", got)
	}
	return rows, nil
}

func (c *Client) execute(ctx context.Context, cmd command.Command) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	attempts := c.attempts
	if attempts <= 0 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		server := c.servers[i%len(c.servers)]
		attemptCtx, cancel := contextForTimeout(ctx, c.opts.Timeout)
		rt, err := c.dialer.DialExHQ(attemptCtx, server, c.opts.Transport)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("%s dial %s: %w", cmd.Operation(), server.Addr(), err)
			continue
		}
		out, err := rt.RoundTrip(attemptCtx, cmd)
		cancel()
		closeErr := rt.Close()
		if err != nil {
			lastErr = fmt.Errorf("%s via %s: %w", cmd.Operation(), server.Addr(), err)
			continue
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return out, nil
	}
	if lastErr == nil {
		lastErr = errors.New("tdx exhq: no attempts executed")
	}
	return nil, lastErr
}

func contextForTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
