# Capability Matrix

| Capability | Go v0 | Notes |
|---|---:|---|
| TCP setup and response frame decode | yes | Includes zlib and raw frame body validation. |
| Server seed list and ping | yes | Static seed list plus `PingAll`. |
| Operation-aware health checks | yes | `HealthCheck` accepts command objects; `OperationStats` exposes per-operation host stats and cooling state. |
| Request-level host failover | yes | `MaxAttempts` rotates hosts and records stats; operation-aware cooldown skips failing host/operation pairs. |
| Per-host connection pool | yes | Successful round-trippers are reused up to `PoolOptions.MaxIdlePerHost`; failed requests discard the connection. |
| Observability hooks | yes | `Observer` receives per-attempt events with operation, host, attempt, latency, error, row count, body size, and connection reuse flag. |
| Metrics collector | yes | `NewMetricsCollector` aggregates attempts, successes, failures, row counts, latency, and last error by operation/host. |
| Idle heartbeat | yes | `KeepAliveManager` closes repeated-failure connections. |
| Security count/list | yes | Raw/unknown fields preserved. `tdx-validate -full-security-list` pages through the complete selected-market list, emits per-page results, checks count parity plus duplicate symbols, and supports page-level retry through `-security-list-page-retries`. |
| Stock/index K-line | yes | Index parser keeps `UpCount` and `DownCount`. |
| Snapshot/quotes | yes | Parser implemented with 5-level book, server time, unknown fields, raw record bytes, and 80-symbol batch splitting. 2026-06-09 live multi-market fixture fixed and validated SH/SZ offset handling. |
| Minute time / transactions | yes | Today/history parsers implemented; `unknown_1`, `NumOrders`, and `UnknownLast` are preserved. 2026-06-09 live minute fixture fixed the real-time symbol prefix skip and removed the prior negative-volume warnings in smoke validation. |
| Market stat | yes | Derived from SH `880005` quote following xmtdx; suspended count is the residual `total-up-down-neutral`. |
| Fund flow | yes | Today flow is derived from paged transaction records with xmtdx amount thresholds. |
| History fund flow | partial | Category 22 parser implemented; client falls back to day-bar dates plus historical transaction aggregation when direct response is empty. Live fixtures still needed. |
| Finance / xdxr | yes | Latest finance parser and xdxr parser implemented; xdxr record headers and share-count volume decoding follow xmtdx fixes. |
| Company info | yes | Category and content commands implemented; content returns raw response bytes after the 12-byte protocol header. |
| Block/report file | partial | Metadata, chunk fetch, report fetch loop, and local `.dat` board parser implemented. 2026-06-09 live `boards_concept` returned rows, but `base_info.zip` returned an empty public-server payload. |
| BJ stable full universe | partial | Public servers are unstable; fallback via report files/base info remains to implement. |
| Raw fixture capture | yes | `Client.Capture`, `tdx-probe -capture-dir`, and `tdx-fixture-matrix` preserve request/header/raw-body/decoded-body/parsed JSON. |
| Python comparison CLI | yes | `tdx-compare-py` compares Go JSON or fixture `parsed_json` against pytdx/xmtdx reference JSON. |
| Live integrity validation | partial | `tdx-validate` runs public API checks and emits JSON reports with per-operation timeout. Latest 2026-06-09 smoke has 0 warnings for SH/SZ core quote/minute checks; remaining failures are public-server timeouts for fund-flow/history-fund-flow plus empty report-file payloads. |
| Benchmarks | yes | Codec, frame, command parser, validation, and quote batch-split benchmarks are implemented with `go test -run=^$ -bench=. -benchmem ./codec ./frame ./command ./validation .`. |
| CLI diagnostics | yes | `tdx-health`, `tdx-probe`, `tdx-fixture-matrix`, `tdx-validate`, `tdx-dump-frame`, and `tdx-compare-py` are implemented; matrix output is JSONL and continues after per-operation failures. |
| Fake TDX fault server | yes | `tdxtest.StartScript` simulates normal frames, raw bytes, bad zlib, partial frames, delayed responses, and disconnects. |
