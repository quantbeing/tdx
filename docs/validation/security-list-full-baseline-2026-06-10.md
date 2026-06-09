# Security List Full Baseline 2026-06-10

## Scope

This records live full-list validation for SZ and BJ after `tdx-validate` gained page-level results and `-security-list-page-retries`.

All commands were run from `/Users/liuhanqing01/projects/tdx` with `TDX_LIVE=1` on 2026-06-10 Asia/Shanghai.

## SZ Baseline

Command:

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 80s \
  -operation-timeout 8s \
  -connect-timeout 1s \
  -markets sz \
  -symbols sz:000001 \
  -kline day \
  -full-security-list \
  -security-list-page-retries 1 \
  -skip-boards \
  -skip-files
```

Result:

- `security_list_SZ_full`: OK
- Rows: `23411`
- Latency: `38739 ms`
- First page smoke: `security_list_SZ_0` OK, `1000` rows
- Retry-success pages:
  - `security_list_SZ_page_5000`
  - `security_list_SZ_page_11000`
  - `security_list_SZ_page_17000`
  - `security_list_SZ_page_23000`

Interpretation:

- SZ full universe can be pulled from public HQ servers with page-level retry enabled.
- The final page was `security_list_SZ_page_23000` with `411` rows, matching the `23411` count.
- The retry pattern mirrors SH: several public-server page requests time out on first attempt but succeed on the next page-level request.

## BJ Baseline

First command:

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 70s \
  -operation-timeout 8s \
  -connect-timeout 1s \
  -markets bj \
  -symbols sh:600519 \
  -kline day \
  -full-security-list \
  -security-list-page-retries 1 \
  -skip-boards \
  -skip-files
```

Second command:

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 100s \
  -operation-timeout 15s \
  -connect-timeout 1s \
  -markets bj \
  -symbols sh:600519 \
  -kline day \
  -full-security-list \
  -security-list-page-retries 3 \
  -skip-boards \
  -skip-files
```

Result from the longer retry run:

- `security_count_BJ`: OK
- Count: `345`
- `security_list_BJ_0`: failed, `0` rows, `15001 ms`
- `security_list_BJ_page_0`: failed, `0` rows, `60006 ms`
- `security_list_BJ_full`: failed, `0/345` rows
- Retry warnings: attempts 1, 2, and 3 all timed out on `security_list_BJ_page_0`

Interpretation:

- BJ count is reachable through HQ `security_count`.
- BJ list page 0 is not reliable through the current public HQ `security_list` path in this network.
- Increasing page retries from 1 to 3 and operation timeout from 8s to 15s did not recover BJ page 0.
- BJ full universe should remain partial until fallback sources are implemented and validated.

## Next Work

- Add BJ fallback collection from protocol-accessible files such as `base_info.zip` or related report/security files.
- Build a host-operation matrix specifically for `security_list_BJ_page_0`.
- Consider exposing full-list retry budgets separately from general operation timeout if longer BJ probing is needed.
- Keep SH/SZ full-list validation in live smoke with `-security-list-page-retries 1`.
