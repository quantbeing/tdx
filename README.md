# tdx

New Go implementation of the TongDaXin HQ TCP protocol.

This repository does not wrap gotdx. It implements the protocol directly with:

- TCP setup and response frame decoding.
- GBK, price-varint, volume, and datetime codecs.
- Core commands for security count/list, stock/index K-lines, realtime five-level quotes, minute time, transactions, finance info, xdxr info, company info, history fund-flow category 22, and block/report files.
- Canonical helpers for market statistics and fund-flow aggregation.
- Request-level host failover and operation health checks.
- Operation-aware circuit breaker with per-operation cooldown and `OperationStats`.
- Per-host idle connection pool; successful connections are reused and failed connections are discarded.
- Request observer hooks and a built-in metrics collector for operation/host latency, failures, retries, and row counts.
- Scriptable fake TDX server for timeout, slow response, partial frame, bad zlib, disconnect, and failover tests.
- Heartbeat manager for long-lived transports.
- Raw byte preservation for protocol auditing.

## Quick Start

```go
client := tdx.NewClient(tdx.Options{})
count, err := client.GetSecurityCount(context.Background(), model.MarketSH)
```

```go
client := tdx.NewClient(tdx.Options{
    Pool: tdx.PoolOptions{
        MaxIdlePerHost: 2,
    },
    CircuitBreaker: tdx.CircuitBreakerOptions{
        FailureThreshold: 2,
        Cooldown: 30 * time.Second,
    },
})
stats := client.OperationStats("security_list")
```

```go
metrics := tdx.NewMetricsCollector()
client := tdx.NewClient(tdx.Options{Observer: metrics})
_ = client.Ping(context.Background())
snapshots := metrics.Snapshot()
```

## Diagnostics

```bash
go run ./cmd/tdx-health
go run ./cmd/tdx-probe -op security-count -market sh
go run ./cmd/tdx-probe -op history-fund-flow -capture-dir ./fixtures/live
TDX_LIVE=1 go run ./cmd/tdx-fixture-matrix -out ./fixtures/live -ops security-count,quote,history-fund-flow
go run ./cmd/tdx-probe -op quote -capture-dir ./fixtures/live
go run ./cmd/tdx-dump-frame -hex <header-plus-body>
go run ./cmd/tdx-compare-py -go ./fixtures/live/<go.fixture.json> -py ./fixtures/pytdx/<py.json>
```

`tdx-probe -capture-dir` writes request bytes, the 16-byte response header, compressed/raw body bytes, decoded body bytes, and parsed JSON into a fixture file. `tdx-fixture-matrix` runs a live operation matrix and writes one JSONL summary row per operation; it requires `TDX_LIVE=1` unless `-allow-live` is set. `history-fund-flow` captures the category 22 direct response; `fund-flow` probes the transaction source used by client-side aggregation. `tdx-compare-py` compares either normal JSON files or Go fixture `parsed_json` output against pytdx/xmtdx reference JSON.

Live tests are intentionally separate from unit tests. Capture binary fixtures before expanding parsers for fund flow and extended-market commands.

## Fault Tests

`tdxtest.StartScript` can simulate protocol and network failures without touching public servers:

```go
server, _ := tdxtest.StartScript(tdxtest.Script{
    Connections: []tdxtest.ConnectionScript{{
        Actions: []tdxtest.Action{
            tdxtest.ReadAndRespond(nil),
            tdxtest.ReadAndRespond(nil),
            tdxtest.ReadAndRespond(nil),
            tdxtest.ReadAndBadZlib([]byte{1, 2, 3}, 8),
        },
    }},
})
```
