# Examples

These examples are small copyable entry points for projects that consume `github.com/quantbeing/tdx`.

## Quickstart

```bash
go run ./examples/quickstart
```

Shows:

- client construction with fast timeout policy
- one-page `ListASharesWithOptions` sampling for fast smoke tests
- batch quotes
- daily K-line bars
- partial-result handling

This example connects to public TDX HQ servers. Use plain `ListAShares` when you want the complete SH/SZ instrument universe.

## Private Retry Policy

```bash
go run ./examples/private_retry
```

Shows:

- custom private server list
- same-host-first retry for private/transient failures
- per-host connection pool
- request-level override with `WithRequestOptions`

Edit the private server IPs before running.

## Diagnostics

See [diagnostics](./diagnostics) for copyable `tdx-health`, `tdx-probe`, and `tdx-validate` commands.

## Offline Go Examples

The root package also contains `Example...` tests backed by a fake dialer. They do not connect to public servers and are verified by:

```bash
go test .
```
