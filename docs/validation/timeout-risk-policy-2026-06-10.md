# Timeout Risk Policy 2026-06-10

## Purpose

This note records the timeout and retry decision made after the first operation-host matrix run.

The goal is to reduce wasted time on public TDX nodes without making normal successful responses fragile.

## Observed Latency

Source: `/tmp/tdx-op-matrix-smoke.json`, produced by `tdx-op-matrix` against:

- `180.153.18.170:7709`
- `180.153.18.171:7709`
- `115.238.56.198:7709`

| Class | Observed latency | Interpretation |
|---|---:|---|
| Successful operations | max `314ms` | Normal successful public-HQ responses were sub-second in this sample. |
| Bad host connect timeout | about `1000ms` | `180.153.18.171:7709` failed every tested operation. |
| BJ security-list read timeout | about `6000ms` | Connect succeeded on usable hosts, but `security_list + BJ` did not return. |

After implementing the fast-timeout path, a second matrix used:

```bash
go run ./cmd/tdx-op-matrix \
  -allow-live \
  -hosts 180.153.18.170:7709,180.153.18.171:7709,115.238.56.198:7709 \
  -ops security-count,quote,security-list-bj,history-fund-flow,report \
  -repeats 3 \
  -operation-timeout 2s \
  -connect-timeout 700ms \
  -timeout 120s \
  > /tmp/tdx-op-matrix-fast-timeout.json
```

Result duration was `24629ms` for 45 host/operation runs. This is lower than the earlier 30-run matrix duration of `36705ms`, despite doing 50% more runs.

| Operation | Host | Runs | OK | Failed | Avg ms | Max ms | Recommendation |
|---|---|---:|---:|---:|---:|---:|---:|
| `security-count` | `180.153.18.170` | 3 | 3 | 0 | 76 | 96 | `500ms` |
| `security-count` | `115.238.56.198` | 3 | 3 | 0 | 91 | 98 | `500ms` |
| `quote` | `180.153.18.170` | 3 | 3 | 0 | 88 | 122 | `500ms` |
| `quote` | `115.238.56.198` | 3 | 3 | 0 | 86 | 94 | `500ms` |
| `history-fund-flow` | `180.153.18.170` | 3 | 3 | 0 | 62 | 63 | `500ms` |
| `history-fund-flow` | `115.238.56.198` | 3 | 3 | 0 | 96 | 101 | `500ms` |
| `report` | `180.153.18.170` | 3 | 3 | 0 | 66 | 75 | `500ms` |
| `report` | `115.238.56.198` | 3 | 3 | 0 | 127 | 215 | `860ms` |
| `180.153.18.171` failures | all tested ops | 15 | 0 | 15 | about 701 | 702 | about `701ms` |
| `security-list-bj` | `180.153.18.170` / `115.238.56.198` | 6 | 0 | 6 | 2001 | 2001 | `1500ms` |

## Implemented Policy

- Added `tdx.TimeoutPolicy`.
- Added `tdx.FastTimeoutPolicy()` with conservative low-latency per-operation defaults.
- Added market-specific timeout support through `tdx.OperationMarket`.
- Added explicit retry strategy:
  - `RetryStrategyFailoverFirst` is the default.
  - `RetryStrategySameHostFirst` is optional for private/transient-failure environments.
- `MaxAttempts` now means total attempts. If it is greater than the server count, failover-first cycles through servers again.
- `tdx-op-matrix` JSON and JSONL now include `timeout_recommendations`.

## Recommended Production Defaults

| Path | Timeout |
|---|---:|
| TCP connect/write | `500-800ms` |
| quote/count/K-line/minute/finance/XDXR | `1-1.5s` |
| SH/SZ security-list page | `1.5-2.5s` |
| report/block/company/file chunk | `2-3s` |
| transaction/history | `2-3s` |
| BJ security-list | `800ms-1.5s`, then fallback |

## Retry Comparison

The new regression test `TestClientFailoverFirstHasHigherSuccessRateThanSameHostRetryWhenHostIsDown` simulates one permanently bad host and one good host with `MaxAttempts=2`.

| Strategy | Result |
|---|---:|
| failover-first | `6/6` successes |
| same-host-first with 2 same-host attempts | lower than failover-first |

Conclusion: public-node default should remain failover-first. Same-host retry can be exposed but should not be the default because it can spend the entire retry budget on a host that is already unavailable for the operation.

## Recommendation Heuristic

`tdx-op-matrix` now emits timeout recommendations per host/operation:

- If there are successful samples, recommend `max_observed_latency * 4`, clamped by min/max bounds.
- If there are no successful samples, recommend a fail-fast timeout.

These recommendations are diagnostic hints only. Production callers should still keep an outer context deadline for the whole business operation.
