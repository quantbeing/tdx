package tdx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quantbeing/tdx/command"
	"github.com/quantbeing/tdx/frame"
	"github.com/quantbeing/tdx/model"
)

type Options struct {
	Servers        []model.Server
	Timeout        time.Duration
	MaxAttempts    int
	Dialer         Dialer
	Transport      TransportOptions
	TimeoutPolicy  TimeoutPolicy
	Retry          RetryOptions
	CircuitBreaker CircuitBreakerOptions
	Pool           PoolOptions
	Observer       Observer
}

type RetryStrategy string

const (
	RetryStrategyFailoverFirst RetryStrategy = "failover_first"
	RetryStrategySameHostFirst RetryStrategy = "same_host_first"
)

type RetryOptions struct {
	Strategy         RetryStrategy
	SameHostAttempts int
}

type OperationMarket struct {
	Operation string
	Market    model.Market
}

type TimeoutPolicy struct {
	DefaultTimeout          time.Duration
	OperationTimeouts       map[string]time.Duration
	MarketOperationTimeouts map[OperationMarket]time.Duration
}

type TransportOptions struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

type PoolOptions struct {
	MaxIdlePerHost int
	Disable        bool
}

type CircuitBreakerOptions struct {
	FailureThreshold int
	Cooldown         time.Duration
}

const DefaultFileChunkSize = 30000
const MaxFileChunks = 256

var (
	errChunkBudgetExceeded = errors.New("tdx chunk budget exceeded")
	errPageBudgetExceeded  = errors.New("tdx page budget exceeded")
	errPartialResult       = errors.New("tdx partial result")
)

// IsChunkBudgetError reports whether err was caused by a chunk budget limit.
func IsChunkBudgetError(err error) bool {
	return errors.Is(err, errChunkBudgetExceeded)
}

// IsPageBudgetError reports whether err was caused by a pagination budget limit.
func IsPageBudgetError(err error) bool {
	return errors.Is(err, errPageBudgetExceeded)
}

// IsBudgetError reports whether err was caused by a file chunk or transaction page budget.
func IsBudgetError(err error) bool {
	return IsChunkBudgetError(err) || IsPageBudgetError(err)
}

// IsPartialResultError reports whether err accompanies a typed partial result.
func IsPartialResultError(err error) bool {
	return errors.Is(err, errPartialResult)
}

type ListSecuritiesOptions struct {
	Markets           []model.Market
	MaxPagesPerMarket int
	StopOnError       bool
}

// FileFetchOptions bounds chunked server file reads.
type FileFetchOptions struct {
	ChunkSize int
	MaxChunks int
}

// FundFlowOptions bounds transaction pagination used by fund-flow helpers.
type FundFlowOptions struct {
	PageSize int
	MaxStart int
	MaxPages int
}

type Dialer interface {
	DialTDX(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error)
}

type DialerFunc func(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error)

func (f DialerFunc) DialTDX(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error) {
	return f(ctx, server, opts)
}

type RoundTripper interface {
	RoundTrip(ctx context.Context, cmd command.Command) (any, error)
	Close() error
}

type RawRoundTripper interface {
	RoundTripRaw(ctx context.Context, cmd command.Command) (CapturedResponse, error)
}

type CapturedResponse struct {
	Operation   string        `json:"operation"`
	Server      model.Server  `json:"server"`
	Attempt     int           `json:"attempt"`
	Latency     time.Duration `json:"latency"`
	Request     []byte        `json:"request"`
	Header      frame.Header  `json:"header"`
	HeaderBytes []byte        `json:"header_bytes"`
	RawBody     []byte        `json:"raw_body"`
	Body        []byte        `json:"body"`
	Parsed      any           `json:"parsed"`
}

type Client struct {
	mu               sync.Mutex
	servers          []model.Server
	stats            []model.ServerStat
	opStats          map[string][]model.ServerStat
	opFailures       map[string][]int
	opCoolUntil      map[string][]time.Time
	idle             [][]RoundTripper
	next             int
	dialer           Dialer
	opts             Options
	attempts         int
	breakerFailure   int
	breakerCooldown  time.Duration
	maxIdlePerHost   int
	retryStrategy    RetryStrategy
	sameHostAttempts int
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
	opts.Retry = normalizeRetryOptions(opts.Retry)
	breakerFailure := opts.CircuitBreaker.FailureThreshold
	if breakerFailure <= 0 {
		breakerFailure = 3
	}
	breakerCooldown := opts.CircuitBreaker.Cooldown
	if breakerCooldown <= 0 {
		breakerCooldown = 30 * time.Second
	}
	maxIdlePerHost := opts.Pool.MaxIdlePerHost
	if opts.Pool.Disable {
		maxIdlePerHost = 0
	} else if maxIdlePerHost <= 0 {
		maxIdlePerHost = 1
	}
	if len(opts.Servers) == 0 {
		opts.Servers = KnownServers()
	}
	attempts := opts.MaxAttempts
	if attempts <= 0 {
		attempts = len(opts.Servers)
	}
	stats := make([]model.ServerStat, len(opts.Servers))
	for i, s := range opts.Servers {
		stats[i] = model.ServerStat{Server: s, Score: 1}
	}
	return &Client{
		servers:          append([]model.Server(nil), opts.Servers...),
		stats:            stats,
		opStats:          make(map[string][]model.ServerStat),
		opFailures:       make(map[string][]int),
		opCoolUntil:      make(map[string][]time.Time),
		idle:             make([][]RoundTripper, len(opts.Servers)),
		dialer:           opts.Dialer,
		opts:             opts,
		attempts:         attempts,
		breakerFailure:   breakerFailure,
		breakerCooldown:  breakerCooldown,
		maxIdlePerHost:   maxIdlePerHost,
		retryStrategy:    opts.Retry.Strategy,
		sameHostAttempts: opts.Retry.SameHostAttempts,
	}
}

func normalizeRetryOptions(opts RetryOptions) RetryOptions {
	if opts.Strategy == "" {
		opts.Strategy = RetryStrategyFailoverFirst
	}
	if opts.Strategy != RetryStrategySameHostFirst {
		opts.Strategy = RetryStrategyFailoverFirst
	}
	if opts.SameHostAttempts <= 0 {
		opts.SameHostAttempts = 1
	}
	if opts.Strategy == RetryStrategyFailoverFirst {
		opts.SameHostAttempts = 1
	}
	return opts
}

func FastTimeoutPolicy() TimeoutPolicy {
	return TimeoutPolicy{
		DefaultTimeout: 1500 * time.Millisecond,
		OperationTimeouts: map[string]time.Duration{
			"security_count":        time.Second,
			"security_quotes":       time.Second,
			"security_bars":         1500 * time.Millisecond,
			"index_bars":            1500 * time.Millisecond,
			"security_list":         2 * time.Second,
			"minute_time":           1500 * time.Millisecond,
			"history_minute_time":   2 * time.Second,
			"transaction":           2500 * time.Millisecond,
			"history_transaction":   3 * time.Second,
			"history_fund_flow":     3 * time.Second,
			"finance_info":          1500 * time.Millisecond,
			"xdxr_info":             1500 * time.Millisecond,
			"company_info_category": 2 * time.Second,
			"company_info_content":  2 * time.Second,
			"block_info_meta":       2 * time.Second,
			"block_info":            3 * time.Second,
			"report_file":           3 * time.Second,
		},
		MarketOperationTimeouts: map[OperationMarket]time.Duration{
			{Operation: "security_list", Market: model.MarketBJ}: 1200 * time.Millisecond,
		},
	}
}

func (p TimeoutPolicy) TimeoutFor(cmd command.Command) time.Duration {
	if cmd == nil {
		return p.DefaultTimeout
	}
	operation := cmd.Operation()
	if market, ok := commandMarket(cmd); ok && p.MarketOperationTimeouts != nil {
		if timeout := p.MarketOperationTimeouts[OperationMarket{Operation: operation, Market: market}]; timeout > 0 {
			return timeout
		}
	}
	if p.OperationTimeouts != nil {
		if timeout := p.OperationTimeouts[operation]; timeout > 0 {
			return timeout
		}
	}
	return p.DefaultTimeout
}

func commandMarket(cmd command.Command) (model.Market, bool) {
	switch c := cmd.(type) {
	case command.SecurityCountCommand:
		return c.Market, true
	case command.SecurityListCommand:
		return c.Market, true
	case command.SecurityQuotesCommand:
		if len(c.Symbols) == 1 {
			return c.Symbols[0].Market, true
		}
	case command.BarsCommand:
		return c.Market, true
	case command.MinuteTimeDataCommand:
		return c.Market, true
	case command.TransactionDataCommand:
		return c.Market, true
	case command.HistoryFundFlowCommand:
		return c.Market, true
	case command.FinanceInfoCommand:
		return c.Market, true
	case command.XdxrInfoCommand:
		return c.Market, true
	case command.CompanyInfoCategoryCommand:
		return c.Market, true
	case command.CompanyInfoContentCommand:
		return c.Market, true
	}
	return 0, false
}

func KnownServers() []model.Server {
	return []model.Server{
		{Name: "tdx-sh-170", Host: "180.153.18.170", Port: 7709},
		{Name: "tdx-sh-171", Host: "180.153.18.171", Port: 7709},
		{Name: "tdx-sh-172", Host: "180.153.18.172", Port: 7709},
		{Name: "tdx-hz-198", Host: "115.238.56.198", Port: 7709},
		{Name: "tdx-hz-165", Host: "115.238.90.165", Port: 7709},
		{Name: "tdx-sz-81", Host: "119.147.212.81", Port: 7709},
		{Name: "tdx-qb-14", Host: "123.125.108.14", Port: 7709},
		{Name: "tdx-qb-114", Host: "110.41.147.114", Port: 7709},
		{Name: "tdx-qb-72", Host: "110.41.2.72", Port: 7709},
	}
}

func FromBestHost(ctx context.Context, opts Options) (*Client, error) {
	results := PingAll(ctx, opts.Servers, opts.Transport)
	if len(results) == 0 {
		return NewClient(opts), errors.New("tdx: no reachable hosts during ping")
	}
	opts.Servers = []model.Server{results[0].Server}
	return NewClient(opts), nil
}

type HostHealth struct {
	Server  model.Server      `json:"server"`
	OK      bool              `json:"ok"`
	Latency time.Duration     `json:"latency"`
	Checks  []OperationHealth `json:"checks,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func FromBestHostByOperations(ctx context.Context, opts Options, probes ...command.Command) (*Client, []HostHealth, error) {
	servers := opts.Servers
	if len(servers) == 0 {
		servers = KnownServers()
	}
	if len(probes) == 0 {
		probes = []command.Command{command.NewSecurityCountCommand(model.MarketSH)}
	}
	ch := make(chan HostHealth, len(servers))
	for _, server := range servers {
		server := server
		go func() {
			ch <- probeHostOperations(ctx, opts, server, probes)
		}()
	}
	health := make([]HostHealth, 0, len(servers))
	for range servers {
		health = append(health, <-ch)
	}
	sortHostHealth(health)
	for _, item := range health {
		if item.OK {
			opts.Servers = []model.Server{item.Server}
			return NewClient(opts), health, nil
		}
	}
	if len(opts.Servers) == 0 {
		opts.Servers = servers
	}
	return NewClient(opts), health, errors.New("tdx: no host passed operation health check")
}

type PingResult struct {
	Server  model.Server  `json:"server"`
	Latency time.Duration `json:"latency"`
	Error   string        `json:"error,omitempty"`
}

func PingAll(ctx context.Context, servers []model.Server, opts TransportOptions) []PingResult {
	if len(servers) == 0 {
		servers = KnownServers()
	}
	out := make(chan PingResult, len(servers))
	for _, server := range servers {
		server := server
		go func() {
			start := time.Now()
			rt, err := NetDialer{}.DialTDX(ctx, server, opts)
			if err != nil {
				out <- PingResult{Server: server, Error: err.Error()}
				return
			}
			_ = rt.Close()
			out <- PingResult{Server: server, Latency: time.Since(start)}
		}()
	}
	results := make([]PingResult, 0, len(servers))
	for range servers {
		res := <-out
		if res.Error == "" {
			results = append(results, res)
		}
	}
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Latency < results[i].Latency {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results
}

func (c *Client) Close() error {
	conns := c.drainIdle()
	var firstErr error
	for _, rt := range conns {
		if err := rt.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Client) SetServers(servers []model.Server) {
	for _, rt := range c.drainIdle() {
		_ = rt.Close()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers = append([]model.Server(nil), servers...)
	c.stats = make([]model.ServerStat, len(servers))
	for i, s := range servers {
		c.stats[i] = model.ServerStat{Server: s, Score: 1}
	}
	c.opStats = make(map[string][]model.ServerStat)
	c.opFailures = make(map[string][]int)
	c.opCoolUntil = make(map[string][]time.Time)
	c.idle = make([][]RoundTripper, len(servers))
	c.next = 0
	if c.opts.MaxAttempts <= 0 {
		c.attempts = len(servers)
	} else {
		c.attempts = c.opts.MaxAttempts
	}
}

func (c *Client) ServerStats() []model.ServerStat {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshGlobalCoolingLocked(time.Now())
	return append([]model.ServerStat(nil), c.stats...)
}

func (c *Client) OperationStats(op string) []model.ServerStat {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := c.ensureOperationLocked(op)
	c.refreshOperationCoolingLocked(op, time.Now())
	return append([]model.ServerStat(nil), stats...)
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.GetSecurityCount(ctx, model.MarketSH)
	return err
}

func (c *Client) HealthCheck(ctx context.Context, ops ...command.Command) []OperationHealth {
	if len(ops) == 0 {
		ops = []command.Command{command.NewSecurityCountCommand(model.MarketSH)}
	}
	out := make([]OperationHealth, 0, len(ops))
	for _, op := range ops {
		start := time.Now()
		_, err := c.execute(ctx, op)
		h := OperationHealth{Operation: op.Operation(), Latency: time.Since(start), OK: err == nil}
		if err != nil {
			h.Error = err.Error()
		}
		out = append(out, h)
	}
	return out
}

func probeHostOperations(ctx context.Context, opts Options, server model.Server, probes []command.Command) HostHealth {
	probeOpts := opts
	probeOpts.Servers = []model.Server{server}
	probeOpts.MaxAttempts = 1
	probeOpts.Pool.Disable = true
	client := NewClient(probeOpts)
	defer client.Close()

	start := time.Now()
	checks := client.HealthCheck(ctx, probes...)
	health := HostHealth{Server: server, Checks: checks, Latency: time.Since(start), OK: len(checks) > 0}
	for _, check := range checks {
		if !check.OK {
			health.OK = false
			health.Error = check.Error
			break
		}
	}
	if len(checks) == 0 {
		health.Error = "no health checks executed"
	}
	return health
}

func sortHostHealth(health []HostHealth) {
	for i := 0; i < len(health); i++ {
		for j := i + 1; j < len(health); j++ {
			if hostHealthLess(health[j], health[i]) {
				health[i], health[j] = health[j], health[i]
			}
		}
	}
}

func hostHealthLess(a HostHealth, b HostHealth) bool {
	if a.OK != b.OK {
		return a.OK
	}
	if a.Latency != b.Latency {
		return a.Latency < b.Latency
	}
	return a.Server.Addr() < b.Server.Addr()
}

func (c *Client) Capture(ctx context.Context, cmd command.Command) (CapturedResponse, error) {
	var lastErr error
	attempts := c.attempts
	if attempts <= 0 {
		attempts = 1
	}
	var plan retryAttemptPlan
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := c.contextForCommand(ctx, cmd)
		idx, server := c.pickAttemptServer(cmd.Operation(), &plan)
		rt, reused, err := c.acquire(attemptCtx, idx, server)
		if err != nil {
			cancel()
			lastErr = err
			c.report(idx, cmd.Operation(), err, 0)
			c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: false, Error: err.Error()})
			continue
		}
		rawRT, ok := rt.(RawRoundTripper)
		if !ok {
			cancel()
			c.release(idx, rt, false)
			err := fmt.Errorf("tdx round tripper for %s does not support capture", server.Addr())
			lastErr = err
			c.report(idx, cmd.Operation(), err, 0)
			c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: false, Error: err.Error(), Reused: reused})
			continue
		}
		start := time.Now()
		out, err := rawRT.RoundTripRaw(attemptCtx, cmd)
		latency := time.Since(start)
		if err != nil {
			cancel()
			c.release(idx, rt, false)
			lastErr = fmt.Errorf("%s capture via %s attempt %d: %w", cmd.Operation(), server.Addr(), attempt+1, err)
			c.report(idx, cmd.Operation(), err, latency)
			c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: false, Error: err.Error(), Latency: latency, Reused: reused})
			continue
		}
		cancel()
		c.release(idx, rt, true)
		out.Operation = cmd.Operation()
		out.Server = server
		out.Attempt = attempt + 1
		out.Latency = latency
		c.report(idx, cmd.Operation(), nil, latency)
		c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: true, Latency: latency, Rows: rowCount(out.Parsed), BodySize: len(out.Body), Reused: reused})
		return out, nil
	}
	if lastErr == nil {
		lastErr = errors.New("tdx: no capture attempts executed")
	}
	return CapturedResponse{}, lastErr
}

type OperationHealth struct {
	Operation string        `json:"operation"`
	OK        bool          `json:"ok"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
}

func (c *Client) GetSecurityCount(ctx context.Context, market model.Market) (uint16, error) {
	got, err := c.execute(ctx, command.NewSecurityCountCommand(market))
	if err != nil {
		return 0, err
	}
	count, ok := got.(uint16)
	if !ok {
		return 0, fmt.Errorf("tdx security_count unexpected reply %T", got)
	}
	return count, nil
}

func (c *Client) GetSecurityList(ctx context.Context, market model.Market, start int) ([]model.Security, error) {
	got, err := c.execute(ctx, command.NewSecurityListCommand(market, start))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.Security)
	if !ok {
		return nil, fmt.Errorf("tdx security_list unexpected reply %T", got)
	}
	return items, nil
}

func (c *Client) ListSecurities(ctx context.Context, markets ...model.Market) (model.PartialResult[model.Security], error) {
	return c.ListSecuritiesWithOptions(ctx, ListSecuritiesOptions{Markets: markets})
}

func (c *Client) ListSecuritiesWithOptions(ctx context.Context, opts ListSecuritiesOptions) (model.PartialResult[model.Security], error) {
	markets := opts.Markets
	if len(markets) == 0 {
		markets = []model.Market{model.MarketSH, model.MarketSZ, model.MarketBJ}
	}
	var result model.PartialResult[model.Security]
	for _, market := range markets {
		if err := contextError(ctx); err != nil {
			return result, err
		}
		count, err := c.GetSecurityCount(ctx, market)
		if err != nil {
			result.Failures = append(result.Failures, model.OperationError{Operation: "security_count", Market: market, Err: err.Error()})
			if opts.StopOnError {
				break
			}
			continue
		}
		pages := 0
		for start := 0; start < int(count); start += 1000 {
			if err := contextError(ctx); err != nil {
				return result, err
			}
			if opts.MaxPagesPerMarket > 0 && pages >= opts.MaxPagesPerMarket {
				result.Failures = append(result.Failures, model.OperationError{
					Operation: "security_list_budget",
					Market:    market,
					Start:     start,
					Count:     1000,
					Err:       fmt.Sprintf("max pages per market reached: %d", opts.MaxPagesPerMarket),
				})
				if opts.StopOnError {
					return result, partialResultError(len(result.Failures))
				}
				break
			}
			items, err := c.GetSecurityList(ctx, market, start)
			if err != nil {
				result.Failures = append(result.Failures, model.OperationError{Operation: "security_list", Market: market, Start: start, Count: 1000, Err: err.Error()})
				if opts.StopOnError {
					return result, partialResultError(len(result.Failures))
				}
				break
			}
			pages++
			if len(items) == 0 {
				break
			}
			result.Items = append(result.Items, items...)
			if len(items) < 1000 {
				break
			}
		}
	}
	if len(result.Failures) > 0 {
		return result, partialResultError(len(result.Failures))
	}
	return result, nil
}

func (c *Client) ListAShares(ctx context.Context) (model.PartialResult[model.Security], error) {
	return c.ListASharesWithOptions(ctx, ListSecuritiesOptions{Markets: defaultAShareMarkets()})
}

func (c *Client) ListASharesWithOptions(ctx context.Context, opts ListSecuritiesOptions) (model.PartialResult[model.Security], error) {
	if len(opts.Markets) == 0 {
		opts.Markets = defaultAShareMarkets()
	}
	all, err := c.ListSecuritiesWithOptions(ctx, opts)
	filtered := all.Items[:0]
	for _, sec := range all.Items {
		if isAshare(sec.Market, sec.Code) {
			filtered = append(filtered, sec)
		}
	}
	all.Items = filtered
	return all, err
}

func (c *Client) ListMarkets(context.Context) []model.Market {
	return []model.Market{model.MarketSH, model.MarketSZ, model.MarketBJ}
}

func defaultAShareMarkets() []model.Market {
	return []model.Market{model.MarketSH, model.MarketSZ}
}

func (c *Client) GetSecurityBars(ctx context.Context, market model.Market, code string, category model.KlineCategory, start, count int) ([]model.Bar, error) {
	got, err := c.execute(ctx, command.NewSecurityBarsCommand(market, code, category, start, count))
	if err != nil {
		return nil, err
	}
	return got.([]model.Bar), nil
}

func (c *Client) GetIndexBars(ctx context.Context, market model.Market, code string, category model.KlineCategory, start, count int) ([]model.Bar, error) {
	got, err := c.execute(ctx, command.NewIndexBarsCommand(market, code, category, start, count))
	if err != nil {
		return nil, err
	}
	return got.([]model.Bar), nil
}

func (c *Client) GetBars(ctx context.Context, market model.Market, code string, category model.KlineCategory, start, count int) ([]model.Bar, error) {
	if isIndexLike(market, code) {
		return c.GetIndexBars(ctx, market, code, category, start, count)
	}
	return c.GetSecurityBars(ctx, market, code, category, start, count)
}

func (c *Client) GetSecurityQuotes(ctx context.Context, symbols []model.Symbol) ([]model.Quote, error) {
	out := make([]model.Quote, 0, len(symbols))
	for start := 0; start < len(symbols); start += command.MaxQuoteBatch {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		end := start + command.MaxQuoteBatch
		if end > len(symbols) {
			end = len(symbols)
		}
		got, err := c.execute(ctx, command.NewSecurityQuotesCommand(symbols[start:end]))
		if err != nil {
			return nil, err
		}
		quotes, ok := got.([]model.Quote)
		if !ok {
			return nil, fmt.Errorf("tdx security_quotes unexpected reply %T", got)
		}
		out = append(out, quotes...)
	}
	return out, nil
}

func (c *Client) GetSnapshot(ctx context.Context, symbols []model.Symbol) ([]model.Quote, error) {
	return c.GetSecurityQuotes(ctx, symbols)
}

func (c *Client) GetMinuteTimeData(ctx context.Context, market model.Market, code string) ([]model.MinuteTime, error) {
	got, err := c.execute(ctx, command.NewMinuteTimeDataCommand(market, code))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.MinuteTime)
	if !ok {
		return nil, fmt.Errorf("tdx minute_time unexpected reply %T", got)
	}
	return items, nil
}
func (c *Client) GetHistoryMinuteTimeData(ctx context.Context, market model.Market, code string, date int) ([]model.MinuteTime, error) {
	got, err := c.execute(ctx, command.NewHistoryMinuteTimeDataCommand(market, code, date))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.MinuteTime)
	if !ok {
		return nil, fmt.Errorf("tdx history_minute_time unexpected reply %T", got)
	}
	return items, nil
}
func (c *Client) GetTransactionData(ctx context.Context, market model.Market, code string, start int, count int) ([]model.Transaction, error) {
	got, err := c.execute(ctx, command.NewTransactionDataCommand(market, code, start, count))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.Transaction)
	if !ok {
		return nil, fmt.Errorf("tdx transaction unexpected reply %T", got)
	}
	return items, nil
}
func (c *Client) GetHistoryTransactionData(ctx context.Context, market model.Market, code string, date int, start int, count int) ([]model.Transaction, error) {
	got, err := c.execute(ctx, command.NewHistoryTransactionDataCommand(market, code, date, start, count))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.Transaction)
	if !ok {
		return nil, fmt.Errorf("tdx history_transaction unexpected reply %T", got)
	}
	return items, nil
}
func (c *Client) GetMarketStat(ctx context.Context) (model.MarketStat, error) {
	quotes, err := c.GetSecurityQuotes(ctx, []model.Symbol{{Market: model.MarketSH, Code: "880005"}})
	if err != nil {
		return model.MarketStat{}, err
	}
	if len(quotes) == 0 {
		return model.MarketStat{}, fmt.Errorf("tdx market_stat empty quote response")
	}
	q := quotes[0]
	up := int(q.Price.Float64())
	down := int(q.PreClose.Float64())
	neutral := int(q.Low.Float64())
	total := int(q.High.Float64())
	suspended := total - up - down - neutral
	if suspended < 0 {
		suspended = 0
	}
	return model.MarketStat{
		UpCount: up, DownCount: down, NeutralCount: neutral, SuspendedCount: suspended,
		TotalCount: total, TotalAmount: q.Amount, TotalVolume: q.Vol,
	}, nil
}
func (c *Client) GetFundFlow(ctx context.Context, market model.Market, code string) (model.FundFlow, error) {
	return c.GetFundFlowWithOptions(ctx, market, code, FundFlowOptions{})
}

func (c *Client) GetFundFlowWithOptions(ctx context.Context, market model.Market, code string, opts FundFlowOptions) (model.FundFlow, error) {
	opts = normalizeFundFlowOptions(opts, 2000)
	records, err := c.collectTransactionRecords(ctx, func(start int, count int) ([]model.Transaction, error) {
		return c.GetTransactionData(ctx, market, code, start, count)
	}, opts)
	if err != nil {
		return model.FundFlow{}, err
	}
	return classifyFundFlow(records), nil
}
func (c *Client) GetHistoryFundFlow(ctx context.Context, market model.Market, code string, start int, count int) ([]model.HistoricalFundFlow, error) {
	return c.GetHistoryFundFlowWithOptions(ctx, market, code, start, count, FundFlowOptions{})
}

func (c *Client) GetHistoryFundFlowWithOptions(ctx context.Context, market model.Market, code string, start int, count int, opts FundFlowOptions) ([]model.HistoricalFundFlow, error) {
	opts = normalizeFundFlowOptions(opts, 800)
	got, err := c.execute(ctx, command.NewHistoryFundFlowCommand(market, code, start, count))
	if err == nil {
		if rows, ok := got.([]model.HistoricalFundFlow); ok && len(rows) > 0 {
			return rows, nil
		}
	} else if isContextErr(err) {
		return nil, err
	}
	bars, err := c.GetSecurityBars(ctx, market, code, model.KlineDay, start, count)
	if err != nil {
		return nil, err
	}
	out := make([]model.HistoricalFundFlow, 0, len(bars))
	for _, bar := range bars {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		date := bar.Year*10000 + bar.Month*100 + bar.Day
		records, err := c.collectTransactionRecords(ctx, func(pageStart int, pageSize int) ([]model.Transaction, error) {
			return c.GetHistoryTransactionData(ctx, market, code, date, pageStart, pageSize)
		}, opts)
		if err != nil {
			return nil, err
		}
		flow := classifyFundFlow(records)
		out = append(out, model.HistoricalFundFlow{
			Year: date / 10000, Month: (date / 100) % 100, Day: date % 100,
			SuperIn: flow.SuperIn, SuperOut: flow.SuperOut, LargeIn: flow.LargeIn, LargeOut: flow.LargeOut,
			MediumIn: flow.MediumIn, MediumOut: flow.MediumOut, SmallIn: flow.SmallIn, SmallOut: flow.SmallOut,
		})
	}
	return out, nil
}
func (c *Client) GetFinanceInfo(ctx context.Context, market model.Market, code string) (model.FinanceInfo, error) {
	got, err := c.execute(ctx, command.NewFinanceInfoCommand(market, code))
	if err != nil {
		return model.FinanceInfo{}, err
	}
	info, ok := got.(model.FinanceInfo)
	if !ok {
		return model.FinanceInfo{}, fmt.Errorf("tdx finance_info unexpected reply %T", got)
	}
	return info, nil
}
func (c *Client) GetXdxrInfo(ctx context.Context, market model.Market, code string) ([]model.XdxrRecord, error) {
	got, err := c.execute(ctx, command.NewXdxrInfoCommand(market, code))
	if err != nil {
		return nil, err
	}
	rows, ok := got.([]model.XdxrRecord)
	if !ok {
		return nil, fmt.Errorf("tdx xdxr_info unexpected reply %T", got)
	}
	return rows, nil
}
func (c *Client) GetCompanyInfoCategory(ctx context.Context, market model.Market, code string) ([]model.CompanyInfoCategory, error) {
	got, err := c.execute(ctx, command.NewCompanyInfoCategoryCommand(market, code))
	if err != nil {
		return nil, err
	}
	items, ok := got.([]model.CompanyInfoCategory)
	if !ok {
		return nil, fmt.Errorf("tdx company_info_category unexpected reply %T", got)
	}
	return items, nil
}
func (c *Client) GetCompanyInfoContent(ctx context.Context, market model.Market, code string, filename string, offset int, length int) ([]byte, error) {
	got, err := c.execute(ctx, command.NewCompanyInfoContentCommand(market, code, filename, offset, length))
	if err != nil {
		return nil, err
	}
	content, ok := got.([]byte)
	if !ok {
		return nil, fmt.Errorf("tdx company_info_content unexpected reply %T", got)
	}
	return content, nil
}
func (c *Client) GetBlockInfo(ctx context.Context, filename string) ([]model.Board, error) {
	return c.GetBlockInfoWithOptions(ctx, filename, FileFetchOptions{})
}

func (c *Client) GetBlockInfoWithOptions(ctx context.Context, filename string, opts FileFetchOptions) ([]model.Board, error) {
	opts = normalizeFileFetchOptions(opts)
	got, err := c.execute(ctx, command.NewBlockInfoMetaCommand(filename))
	if err != nil {
		return nil, err
	}
	meta, ok := got.(model.FileMeta)
	if !ok {
		return nil, fmt.Errorf("tdx block_info_meta unexpected reply %T", got)
	}
	if meta.Size <= 0 {
		return nil, fmt.Errorf("tdx block_info %q empty metadata: size=%d", filename, meta.Size)
	}
	neededChunks := (meta.Size + opts.ChunkSize - 1) / opts.ChunkSize
	if neededChunks > opts.MaxChunks {
		return nil, fmt.Errorf("%w: block_info %q needed_chunks=%d max_chunks=%d chunk_size=%d meta_size=%d", errChunkBudgetExceeded, filename, neededChunks, opts.MaxChunks, opts.ChunkSize, meta.Size)
	}
	data := make([]byte, 0, meta.Size)
	for start := 0; start < meta.Size; start += opts.ChunkSize {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		length := opts.ChunkSize
		if remain := meta.Size - start; remain < length {
			length = remain
		}
		chunkAny, err := c.execute(ctx, command.NewBlockInfoCommand(filename, start, length))
		if err != nil {
			return nil, err
		}
		chunk, ok := chunkAny.([]byte)
		if !ok {
			return nil, fmt.Errorf("tdx block_info unexpected reply %T", chunkAny)
		}
		data = append(data, chunk...)
		if len(chunk) < length {
			break
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("tdx block_info %q empty payload", filename)
	}
	boards := command.ParseBlockData(data, filename)
	if len(boards) == 0 {
		return nil, fmt.Errorf("tdx block_info %q invalid payload: bytes=%d meta_size=%d", filename, len(data), meta.Size)
	}
	return boards, nil
}

func (c *Client) GetReportFile(ctx context.Context, filename string) ([]byte, error) {
	return c.GetReportFileWithOptions(ctx, filename, FileFetchOptions{})
}

func (c *Client) GetReportFileWithOptions(ctx context.Context, filename string, opts FileFetchOptions) ([]byte, error) {
	opts = normalizeFileFetchOptions(opts)
	data := make([]byte, 0)
	complete := false
	for i := 0; i < opts.MaxChunks; i++ {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		start := i * opts.ChunkSize
		got, err := c.execute(ctx, command.NewReportFileCommand(filename, start, opts.ChunkSize))
		if err != nil {
			return nil, err
		}
		chunk, ok := got.([]byte)
		if !ok {
			return nil, fmt.Errorf("tdx report_file unexpected reply %T", got)
		}
		data = append(data, chunk...)
		if len(chunk) < opts.ChunkSize {
			complete = true
			break
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("tdx report_file %q empty payload", filename)
	}
	if !complete {
		return nil, fmt.Errorf("%w: report_file %q max_chunks=%d chunk_size=%d bytes=%d", errChunkBudgetExceeded, filename, opts.MaxChunks, opts.ChunkSize, len(data))
	}
	return data, nil
}

func (c *Client) ListBoards(ctx context.Context, boardType string) ([]model.Board, error) {
	return c.ListBoardsWithOptions(ctx, boardType, FileFetchOptions{})
}

func (c *Client) ListBoardsWithOptions(ctx context.Context, boardType string, opts FileFetchOptions) ([]model.Board, error) {
	filename := "block_zs.dat"
	switch boardType {
	case "concept":
		filename = "block_gn.dat"
	case "style":
		filename = "block_fg.dat"
	case "industry", "index":
		filename = "block_zs.dat"
	}
	return c.GetBlockInfoWithOptions(ctx, filename, opts)
}

func (c *Client) ListBoardMembers(ctx context.Context, boardCode string) ([]string, error) {
	return c.ListBoardMembersWithOptions(ctx, boardCode, FileFetchOptions{})
}

func (c *Client) ListBoardMembersWithOptions(ctx context.Context, boardCode string, opts FileFetchOptions) ([]string, error) {
	for _, filename := range []string{"block_gn.dat", "block_fg.dat", "block_zs.dat"} {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		boards, err := c.GetBlockInfoWithOptions(ctx, filename, opts)
		if err != nil {
			if isContextErr(err) || errors.Is(err, errChunkBudgetExceeded) {
				return nil, err
			}
			continue
		}
		for _, board := range boards {
			if board.Name == boardCode {
				return board.Codes, nil
			}
		}
	}
	return nil, fmt.Errorf("tdx board %q not found", boardCode)
}

type transactionSignature struct {
	Hour        int
	Minute      int
	PriceValue  int64
	PriceScale  int32
	Vol         int
	BuyOrSell   int
	UnknownLast int
}

type pageSignature struct {
	First transactionSignature
	Last  transactionSignature
}

func (c *Client) collectTransactionRecords(ctx context.Context, fetch func(start int, count int) ([]model.Transaction, error), opts FundFlowOptions) ([]model.Transaction, error) {
	out := make([]model.Transaction, 0)
	seenRecords := make(map[transactionSignature]struct{})
	seenPages := make(map[pageSignature]struct{})
	pages := 0
	for start := 0; start < opts.MaxStart; {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		if opts.MaxPages > 0 && pages >= opts.MaxPages {
			return nil, fmt.Errorf("%w: transaction max_pages=%d page_size=%d max_start=%d rows=%d", errPageBudgetExceeded, opts.MaxPages, opts.PageSize, opts.MaxStart, len(out))
		}
		records, err := fetch(start, opts.PageSize)
		if err != nil {
			return nil, err
		}
		pages++
		if len(records) == 0 {
			break
		}
		pageSig := pageSignature{First: signatureOfTransaction(records[0]), Last: signatureOfTransaction(records[len(records)-1])}
		if _, ok := seenPages[pageSig]; ok {
			break
		}
		seenPages[pageSig] = struct{}{}

		newCount := 0
		for _, record := range records {
			sig := signatureOfTransaction(record)
			if _, ok := seenRecords[sig]; ok {
				continue
			}
			seenRecords[sig] = struct{}{}
			out = append(out, record)
			newCount++
		}
		if newCount == 0 {
			break
		}
		start += len(records)
		if len(records) < 100 {
			break
		}
	}
	return out, nil
}

func normalizeFileFetchOptions(opts FileFetchOptions) FileFetchOptions {
	if opts.ChunkSize <= 0 || opts.ChunkSize > DefaultFileChunkSize {
		opts.ChunkSize = DefaultFileChunkSize
	}
	if opts.MaxChunks <= 0 {
		opts.MaxChunks = MaxFileChunks
	}
	return opts
}

func normalizeFundFlowOptions(opts FundFlowOptions, defaultPageSize int) FundFlowOptions {
	if opts.PageSize <= 0 {
		opts.PageSize = defaultPageSize
	}
	if opts.MaxStart <= 0 {
		opts.MaxStart = 10000
	}
	return opts
}

func partialResultError(failures int) error {
	return fmt.Errorf("%w: list securities partial failures=%d", errPartialResult, failures)
}

func signatureOfTransaction(record model.Transaction) transactionSignature {
	return transactionSignature{
		Hour: record.Hour, Minute: record.Minute, PriceValue: record.Price.Mantissa, PriceScale: record.Price.Scale,
		Vol: record.Vol, BuyOrSell: record.BuyOrSell, UnknownLast: record.UnknownLast,
	}
}

func classifyFundFlow(records []model.Transaction) model.FundFlow {
	var flow model.FundFlow
	for _, record := range records {
		amount := record.Price.Float64() * float64(record.Vol) * 100
		if amount <= 0 {
			continue
		}
		switch record.BuyOrSell {
		case 0:
			addFundAmount(&flow, amount, true)
		case 1:
			addFundAmount(&flow, amount, false)
		}
	}
	return flow
}

func addFundAmount(flow *model.FundFlow, amount float64, inflow bool) {
	switch {
	case amount > 1_000_000:
		if inflow {
			flow.SuperIn += amount
		} else {
			flow.SuperOut += amount
		}
	case amount > 200_000:
		if inflow {
			flow.LargeIn += amount
		} else {
			flow.LargeOut += amount
		}
	case amount > 40_000:
		if inflow {
			flow.MediumIn += amount
		} else {
			flow.MediumOut += amount
		}
	default:
		if inflow {
			flow.SmallIn += amount
		} else {
			flow.SmallOut += amount
		}
	}
}

func (c *Client) execute(ctx context.Context, cmd command.Command) (any, error) {
	var lastErr error
	attempts := c.attempts
	if attempts <= 0 {
		attempts = 1
	}
	var plan retryAttemptPlan
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := c.contextForCommand(ctx, cmd)
		idx, server := c.pickAttemptServer(cmd.Operation(), &plan)
		rt, reused, err := c.acquire(attemptCtx, idx, server)
		if err != nil {
			cancel()
			lastErr = err
			c.report(idx, cmd.Operation(), err, 0)
			c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: false, Error: err.Error()})
			continue
		}
		start := time.Now()
		out, err := rt.RoundTrip(attemptCtx, cmd)
		if err != nil {
			cancel()
			latency := time.Since(start)
			c.release(idx, rt, false)
			lastErr = fmt.Errorf("%s via %s attempt %d: %w", cmd.Operation(), server.Addr(), attempt+1, err)
			c.report(idx, cmd.Operation(), err, latency)
			c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: false, Error: err.Error(), Latency: latency, Reused: reused})
			continue
		}
		cancel()
		c.release(idx, rt, true)
		latency := time.Since(start)
		c.report(idx, cmd.Operation(), nil, latency)
		c.observe(RequestEvent{Operation: cmd.Operation(), Server: server, Attempt: attempt + 1, OK: true, Latency: latency, Rows: rowCount(out), Reused: reused})
		return out, nil
	}
	if lastErr == nil {
		lastErr = errors.New("tdx: no attempts executed")
	}
	return nil, lastErr
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (c *Client) acquire(ctx context.Context, idx int, server model.Server) (RoundTripper, bool, error) {
	if c.maxIdlePerHost > 0 {
		c.mu.Lock()
		if idx >= 0 && idx < len(c.idle) {
			idle := c.idle[idx]
			if n := len(idle); n > 0 {
				rt := idle[n-1]
				c.idle[idx] = idle[:n-1]
				c.mu.Unlock()
				return rt, true, nil
			}
		}
		c.mu.Unlock()
	}
	rt, err := c.dialer.DialTDX(ctx, server, c.opts.Transport)
	return rt, false, err
}

func (c *Client) observe(event RequestEvent) {
	if c.opts.Observer != nil {
		c.opts.Observer.OnRequest(event)
	}
}

func (c *Client) release(idx int, rt RoundTripper, reusable bool) {
	if rt == nil {
		return
	}
	if !reusable || c.maxIdlePerHost <= 0 {
		_ = rt.Close()
		return
	}
	c.mu.Lock()
	if idx >= 0 && idx < len(c.idle) && len(c.idle[idx]) < c.maxIdlePerHost {
		c.idle[idx] = append(c.idle[idx], rt)
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	_ = rt.Close()
}

func (c *Client) drainIdle() []RoundTripper {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []RoundTripper
	for idx := range c.idle {
		out = append(out, c.idle[idx]...)
		c.idle[idx] = nil
	}
	return out
}

type retryAttemptPlan struct {
	idx       int
	server    model.Server
	remaining int
	ok        bool
}

func (c *Client) pickAttemptServer(op string, plan *retryAttemptPlan) (int, model.Server) {
	if c.retryStrategy == RetryStrategySameHostFirst && c.sameHostAttempts > 1 {
		if plan != nil && plan.ok && plan.remaining > 0 {
			plan.remaining--
			return plan.idx, plan.server
		}
		idx, server := c.pickServer(op)
		if plan != nil {
			plan.idx = idx
			plan.server = server
			plan.remaining = c.sameHostAttempts - 1
			plan.ok = true
		}
		return idx, server
	}
	return c.pickServer(op)
}

func (c *Client) pickServer(op string) (int, model.Server) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.servers) == 0 {
		c.servers = KnownServers()
	}
	now := time.Now()
	c.ensureOperationLocked(op)
	c.refreshOperationCoolingLocked(op, now)
	firstIdx := c.next % len(c.servers)
	for i := 0; i < len(c.servers); i++ {
		idx := c.next % len(c.servers)
		c.next = (idx + 1) % len(c.servers)
		if !c.opStats[op][idx].Cooling {
			return idx, c.servers[idx]
		}
	}
	c.next = (firstIdx + 1) % len(c.servers)
	return firstIdx, c.servers[firstIdx]
}

func (c *Client) contextForCommand(ctx context.Context, cmd command.Command) (context.Context, context.CancelFunc) {
	timeout := c.opts.TimeoutPolicy.TimeoutFor(cmd)
	if timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *Client) report(idx int, op string, err error, latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx < 0 || idx >= len(c.stats) {
		return
	}
	opStats := c.ensureOperationLocked(op)
	c.stats[idx].LastOp = op
	opStats[idx].LastOp = op
	if err != nil {
		c.stats[idx].Failures++
		c.stats[idx].Score *= 0.5
		c.stats[idx].LastError = err.Error()
		opStats[idx].Failures++
		opStats[idx].Score *= 0.5
		opStats[idx].LastError = err.Error()
		c.opFailures[op][idx]++
		if c.opFailures[op][idx] >= c.breakerFailure {
			opStats[idx].Cooling = true
			c.opCoolUntil[op][idx] = time.Now().Add(c.breakerCooldown)
		}
		c.refreshGlobalCoolingLocked(time.Now())
		return
	}
	c.stats[idx].Successes++
	c.stats[idx].LastError = ""
	opStats[idx].Successes++
	opStats[idx].LastError = ""
	opStats[idx].Cooling = false
	c.opFailures[op][idx] = 0
	c.opCoolUntil[op][idx] = time.Time{}
	if c.stats[idx].Score == 0 {
		c.stats[idx].Score = 1
	}
	if opStats[idx].Score == 0 {
		opStats[idx].Score = 1
	}
	if latency > 0 {
		ms := float64(latency.Milliseconds() + 1)
		c.stats[idx].Score = 0.8*c.stats[idx].Score + 0.2*(1000/ms)
		opStats[idx].Score = 0.8*opStats[idx].Score + 0.2*(1000/ms)
	}
	c.refreshGlobalCoolingLocked(time.Now())
}

func (c *Client) ensureOperationLocked(op string) []model.ServerStat {
	if c.opStats == nil {
		c.opStats = make(map[string][]model.ServerStat)
	}
	stats, ok := c.opStats[op]
	if ok && len(stats) == len(c.servers) {
		return stats
	}
	stats = make([]model.ServerStat, len(c.servers))
	for i, server := range c.servers {
		stats[i] = model.ServerStat{Server: server, Score: 1}
	}
	c.opStats[op] = stats
	if c.opFailures == nil {
		c.opFailures = make(map[string][]int)
	}
	if c.opCoolUntil == nil {
		c.opCoolUntil = make(map[string][]time.Time)
	}
	c.opFailures[op] = make([]int, len(c.servers))
	c.opCoolUntil[op] = make([]time.Time, len(c.servers))
	return stats
}

func (c *Client) refreshOperationCoolingLocked(op string, now time.Time) {
	stats := c.ensureOperationLocked(op)
	coolUntil := c.opCoolUntil[op]
	for i := range stats {
		if stats[i].Cooling && !coolUntil[i].IsZero() && !now.Before(coolUntil[i]) {
			stats[i].Cooling = false
			coolUntil[i] = time.Time{}
			c.opFailures[op][i] = 0
		}
	}
}

func (c *Client) refreshGlobalCoolingLocked(now time.Time) {
	for i := range c.stats {
		c.stats[i].Cooling = false
	}
	for op := range c.opStats {
		c.refreshOperationCoolingLocked(op, now)
		for i, stat := range c.opStats[op] {
			if stat.Cooling && i < len(c.stats) {
				c.stats[i].Cooling = true
			}
		}
	}
}

func isAshare(market model.Market, code string) bool {
	return (market == model.MarketSH && hasPrefix(code, "600", "601", "603", "605", "688", "689")) ||
		(market == model.MarketSZ && hasPrefix(code, "000", "001", "002", "003", "300", "301")) ||
		(market == model.MarketBJ && hasPrefix(code, "4", "8", "920"))
}

func isIndexLike(market model.Market, code string) bool {
	return (market == model.MarketSH && hasPrefix(code, "000", "880", "881", "882", "883", "884", "885", "999")) ||
		(market == model.MarketSZ && hasPrefix(code, "395", "399"))
}

func hasPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

type NetDialer struct{}

func (NetDialer) DialTDX(ctx context.Context, server model.Server, opts TransportOptions) (RoundTripper, error) {
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
	out, err := t.RoundTripRaw(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return out.Parsed, nil
}

func (t *tcpRoundTripper) RoundTripRaw(ctx context.Context, cmd command.Command) (CapturedResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	req, err := cmd.BuildRequest()
	if err != nil {
		return CapturedResponse{}, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = t.conn.SetDeadline(deadline)
	} else {
		_ = t.conn.SetDeadline(time.Now().Add(t.opts.ReadTimeout))
	}
	if _, err := t.conn.Write(req); err != nil {
		return CapturedResponse{}, err
	}
	payload, err := readFramePayload(t.conn)
	if err != nil {
		return CapturedResponse{}, err
	}
	parsed, err := cmd.ParseResponse(payload.Body)
	if err != nil {
		return CapturedResponse{}, err
	}
	return CapturedResponse{
		Operation:   cmd.Operation(),
		Request:     append([]byte(nil), req...),
		Header:      payload.Header,
		HeaderBytes: append([]byte(nil), payload.HeaderBytes...),
		RawBody:     append([]byte(nil), payload.RawBody...),
		Body:        append([]byte(nil), payload.Body...),
		Parsed:      parsed,
	}, nil
}

func (t *tcpRoundTripper) setup() error {
	for _, req := range command.SetupCommands {
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
	payload, err := readFramePayload(r)
	if err != nil {
		return nil, err
	}
	return payload.Body, nil
}

type framePayload struct {
	Header      frame.Header
	HeaderBytes []byte
	RawBody     []byte
	Body        []byte
}

func readFramePayload(r io.Reader) (framePayload, error) {
	headerBytes := make([]byte, frame.HeaderSize)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return framePayload{}, err
	}
	header, err := frame.ParseHeader(headerBytes)
	if err != nil {
		return framePayload{}, err
	}
	raw := make([]byte, header.ZipSize)
	if _, err := io.ReadFull(r, raw); err != nil {
		return framePayload{}, err
	}
	body, err := frame.DecodeBody(header, raw)
	if err != nil {
		return framePayload{}, err
	}
	return framePayload{
		Header:      header,
		HeaderBytes: headerBytes,
		RawBody:     raw,
		Body:        body,
	}, nil
}
