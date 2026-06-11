# Diagnostics Examples

These commands are copyable probes for checking public or private TDX HQ servers before using them in production jobs.

## Setup Ping

```bash
go run ./cmd/tdx-health -hosts 180.153.18.170:7709,180.153.18.171:7709 -timeout 1s
```

`tdx-health` without `-probe` only checks TCP/setup connectivity.

## Operation-Aware Probe

```bash
go run ./cmd/tdx-health \
  -hosts 180.153.18.170:7709,180.153.18.171:7709 \
  -probe security-count,security-list-sh,quote \
  -timeout 2s \
  -max-attempts 2 \
  -retry-strategy failover-first
```

This checks whether a host can answer the specific operations your job needs. A server can pass setup ping and still fail `security_list` or quotes.

## Single Operation Probe

```bash
go run ./cmd/tdx-probe \
  -op quote \
  -symbols sh:600519,sz:000001 \
  -timeout 2s \
  -max-attempts 2 \
  -retry-strategy failover-first
```

Use this when you want a compact JSON health result for one command shape.

## Live Validation

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -markets sh,sz \
  -symbols sh:600519,sz:000001 \
  -skip-boards \
  -skip-files \
  -operation-timeout 2s \
  -connect-timeout 800ms \
  -max-attempts 2
```

`tdx-validate` is the broader integrity smoke test. Keep `TDX_LIVE=1` explicit so local CI does not accidentally hit public TDX servers.
