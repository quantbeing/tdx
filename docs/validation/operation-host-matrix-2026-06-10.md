# Operation Host Matrix 2026-06-10

## Scope

This records the first live operation-host matrix run after adding `tdx-op-matrix`.

The purpose is to answer whether public-server failures are concentrated on one host or vary by operation.

Command:

```bash
go run ./cmd/tdx-op-matrix \
  -allow-live \
  -hosts 180.153.18.170:7709,180.153.18.171:7709,115.238.56.198:7709 \
  -ops security-count,quote,security-list-bj,history-fund-flow,report \
  -repeats 2 \
  -operation-timeout 6s \
  -connect-timeout 1s \
  -timeout 180s \
  > /tmp/tdx-op-matrix-smoke.json
```

Result:

- Total host/operation runs: `30`
- Total duration: `36705 ms`
- Hosts: `180.153.18.170:7709`, `180.153.18.171:7709`, `115.238.56.198:7709`
- Repeats per host/operation: `2`

## Summary

| Operation | Host | Runs | OK | Failed | Avg ms | Max ms | Rows | Last error |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `security-count` | `180.153.18.170:7709` | 2 | 2 | 0 | 98 | 112 | 0 | |
| `security-count` | `180.153.18.171:7709` | 2 | 0 | 2 | 1000 | 1001 | 0 | dial tcp timeout |
| `security-count` | `115.238.56.198:7709` | 2 | 2 | 0 | 167 | 168 | 0 | |
| `quote` | `180.153.18.170:7709` | 2 | 2 | 0 | 133 | 168 | 2 | |
| `quote` | `180.153.18.171:7709` | 2 | 0 | 2 | 1000 | 1001 | 0 | dial tcp timeout |
| `quote` | `115.238.56.198:7709` | 2 | 2 | 0 | 165 | 186 | 2 | |
| `security-list-bj` | `180.153.18.170:7709` | 2 | 0 | 2 | 6001 | 6002 | 0 | read timeout |
| `security-list-bj` | `180.153.18.171:7709` | 2 | 0 | 2 | 1001 | 1001 | 0 | dial tcp timeout |
| `security-list-bj` | `115.238.56.198:7709` | 2 | 0 | 2 | 6002 | 6003 | 0 | read timeout |
| `history-fund-flow` | `180.153.18.170:7709` | 2 | 2 | 0 | 270 | 314 | 0 | |
| `history-fund-flow` | `180.153.18.171:7709` | 2 | 0 | 2 | 1005 | 1009 | 0 | dial tcp timeout |
| `history-fund-flow` | `115.238.56.198:7709` | 2 | 2 | 0 | 168 | 216 | 0 | |
| `report` | `180.153.18.170:7709` | 2 | 2 | 0 | 129 | 157 | 0 | empty payload in matrix row |
| `report` | `180.153.18.171:7709` | 2 | 0 | 2 | 1001 | 1001 | 0 | dial tcp timeout |
| `report` | `115.238.56.198:7709` | 2 | 2 | 0 | 197 | 228 | 0 | empty payload in matrix row |

## Interpretation

- Failures are **not all from the same node**.
- `180.153.18.171:7709` failed every tested operation with connect timeout in this run. This is a host-level failure in the current network.
- `security-list-bj` failed on all three tested hosts. On `180.153.18.170` and `115.238.56.198`, connect succeeded but the operation read timed out. This is operation/market-specific instability, not merely one bad node.
- `quote` and `security-count` succeeded on `180.153.18.170` and `115.238.56.198`, confirming those hosts were usable for ordinary HQ operations during the same run.
- `report` returned OK from two hosts but with `0` rows in the matrix result; treat report-file success as transport success only until payload validation is added to the matrix or live validation.

## Next Work

- Extend `tdx-op-matrix` with payload validators for operations where transport success is insufficient, such as report files and history fund flow.
- Run a wider matrix across all `KnownServers()` with `-jsonl` and archive the result when debugging public-server health.
- Consider adding `tdx-validate -op-matrix` or a `--matrix-output` flag so live validation can preserve host/attempt evidence alongside data-integrity findings.
