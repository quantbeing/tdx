# Changelog

## Unreleased

### Added

- Experimental `exhq` package for TDX extended-market `7727` protocol research, with offline-tested parsers and client skeleton.
- Experimental `vipdoc` package for deterministic local `.day` and block `.dat` parsing.
- Experimental `financepkg` package for `tdxfin/gpcw*.zip` professional financial package parsing and report-file download adapters.
- `tdx-probe -raw-hex` diagnostic mode for legal/owned-endpoint raw request fixture capture.

### Notes

- These broader ecosystem packages are not part of the stable HQ `7709` API yet; ExHQ still needs live server validation.
- VIPDOC minute-file parsing remains explicitly unsupported until the local minute record layouts are verified.

## v0.1.2

Tag target for the post-`v0.1.1` hardening work.

### Added

- Request-level policy overrides through `RequestOptions`, `WithRequestOptions`, and `RequestOptionsFromContext`.
- Context-carried overrides for `MaxAttempts`, `Retry`, and `TimeoutPolicy` without adding retry parameters to every data API.
- Defensive copies for timeout-policy maps saved in client options and request contexts.
- Retry/attempt CLI controls for `tdx-validate`, `tdx-probe`, `tdx-fixture-matrix`, and `tdx-health -probe`.
- `tdx-health` tests that keep plain setup ping independent from retry flags.
- Shared internal CLI retry flag parsing helper for consistent retry strategy aliases and errors.
- Root package Go examples backed by a fake dialer.
- Copyable API, private-retry, and diagnostics examples under `examples/`.

### Changed

- `FromBestHostByOperations` still defaults single-host probes to one attempt, but now respects an explicit `Options.MaxAttempts`.
- `tdx-op-matrix` intentionally remains single-attempt per host to preserve host/operation evidence.
- README and capability docs now describe request-level policy layering and CLI retry flags.

### Verified

- `go test -count=1 ./...`
- `go vet ./...`
- `go test -race .`
- `go test -run=^$ -bench=Client -benchmem .`

## v0.1.1

### Added

- Detailed protocol-to-public-API mapping in `docs/api/protocol-interface-map.md`.

### Changed

- `ListAShares` defaults to stable SH/SZ markets; BJ is explicit opt-in through `ListASharesWithOptions`.
- K-line constants are declared and documented by duration order while preserving TDX wire values.

## v0.1.0

Initial reusable HQ `7709` Go client release, including:

- TCP setup, frame decode, zlib, GBK, price/volume/datetime codecs.
- Security count/list, K-line, quote/snapshot, minute, transaction, finance, XDXR, company, block/report file, and derived fund-flow APIs.
- Host failover, operation-aware stats/cooldown, connection pooling, keepalive, observer hooks, and metrics collector.
- Diagnostic CLIs, live validation tooling, fake TDX server, fixtures, benchmarks, and Python comparison support.
