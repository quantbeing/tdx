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

## Official Data Package Probe

After the BJ HQ list timeout, `tdx-data-probe` was added to inspect the official TDX HTTP data package surface.

Manifest command:

```bash
go run ./cmd/tdx-data-probe -timeout 15s -limit 8
```

Result:

- Source: `https://data.tdx.com.cn/tdxgp/gpszsh.txt`
- Entry count: `7240`
- Total declared size: `1538883256`
- Local files: `1`
- Dat files: `7239`
- Skipped lines: `0`

BJ manifest filter:

```bash
go run ./cmd/tdx-data-probe -timeout 15s -prefix gpbj -limit 8
```

Result:

- Entry count: `319`
- Dat files: `319`
- Total declared size: `30113577`
- First returned candidates include `gpbj920992.dat`, `gpbj920985.dat`, `gpbj920982.dat`, `gpbj920981.dat`.

Local index filter:

```bash
go run ./cmd/tdx-data-probe -kind local-index -timeout 15s -prefix gpbj -limit 8
```

Result:

- Source: `https://data.tdx.com.cn/tdxgp/gpszsh.local`
- Entry count: `319`
- Dat files: `319`
- Skipped lines: `0`
- First returned candidates include the same `gpbj920xxx.dat` filenames.

Important integrity finding:

- `gpszsh.local` is an INI-style `[MD5]` index, not a securities table.
- Some manifest MD5 values differ from the `.local` index values for the same file, for example `gpbj920985.dat` in this live run.
- At least one sampled `.dat` file had HTTP `Content-Length` differing from manifest `size` by one 13-byte record, and direct file MD5 did not match the manifest MD5.
- Therefore these fields are useful as diagnostic evidence but must not become hard integrity validation until the official update/checksum semantics are understood.
- Go's standard HTTP client received a `text/html` JavaScript challenge for direct `.dat` fetches in this environment. `tdx-data-probe` now rejects this instead of parsing it as binary. Use curl plus `-input` for binary `.dat` samples.

DAT13 local sample command:

```bash
curl -L --max-time 15 -sS \
  https://data.tdx.com.cn/tdxgp/gpbj920021.dat \
  -o /tmp/tdx-gpbj920021.dat

go run ./cmd/tdx-data-probe \
  -kind dat13 \
  -input /tmp/tdx-gpbj920021.dat \
  -limit 6
```

Result:

- File size: `141154`
- Record size: `13`
- Record count: `10858`
- Trailing bytes: `0`
- Marker distribution is mixed, not a single homogeneous row type. Frequent markers include `27` (`1483` rows), `16` (`1422`), `38` (`1066`), `25` (`1063`), and `3/11/12/13` (`804` each).
- Date-like range in raw summary: min `0`, max `20260609`.
- Float32-like field range: min about `-1820.42`, max about `502124.97`.
- `field2_uint32` is non-zero in `3048` rows.
- First records preserve raw hex plus fields named only by observed shape:
  - `20151231`, `field1_float32=17`
  - `20160630`, `field1_float32=36`
  - `20161231`, `field1_float32=55`
  - `20170630`, `field1_float32=181`

Interpretation:

- Official HTTP data packages give a validated BJ candidate-file fallback surface even when HQ `security_list_BJ_page_0` times out.
- They currently enumerate `319` `gpbj*.dat` candidates, while HQ `security_count_BJ` reports `345`; this is useful but not a complete BJ securities list.
- The sampled `.dat` payload is confirmed to be 13-byte record-oriented, but marker distribution shows multiple row classes. Field semantics are still unknown and it does not yet provide names, status, or canonical security metadata.

## Next Work

- Add BJ fallback collection from official data package candidates and protocol-accessible files such as `base_info.zip` or related report/security files.
- Continue parsing sampled `gpbj*.dat` fixtures with tests; group by `marker`, determine field semantics, and check whether names/security metadata are present elsewhere.
- Investigate official data package checksum/update semantics before enforcing MD5 or size.
- Build a host-operation matrix specifically for `security_list_BJ_page_0`.
- Consider exposing full-list retry budgets separately from general operation timeout if longer BJ probing is needed.
- Keep SH/SZ full-list validation in live smoke with `-security-list-page-retries 1`.
