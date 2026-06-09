# Live Fixture Parser Fixes 2026-06-09

## Scope

This note records the live data-integrity follow-up for the Go TDX HQ `7709` library. It focuses on two parser issues found after running `tdx-validate` against public TDX servers:

- multi-market `security_quotes` returned a corrupted second symbol
- live `minute_time` rows decoded implausible prices and negative volumes

The fixes are backed by committed live fixtures and regression tests.

## Captured Fixtures

```text
fixtures/live/2026-06-09-validation-followup/security_quotes_180_153_18_170_7709_20260609T125505.372328000Z.fixture.json
fixtures/live/2026-06-09-validation-followup/minute_time_180_153_18_170_7709_20260609T125517.607682000Z.fixture.json
```

Capture commands:

```bash
TDX_LIVE=1 go run ./cmd/tdx-probe \
  -op quote \
  -symbols sh:600519,sz:000001 \
  -timeout 12s \
  -capture-dir fixtures/live/2026-06-09-validation-followup

TDX_LIVE=1 go run ./cmd/tdx-probe \
  -op minute \
  -market sh \
  -code 600519 \
  -timeout 12s \
  -capture-dir fixtures/live/2026-06-09-validation-followup
```

## Root Cause: Quote Offset Shadowing

The `security_quotes` parser decoded the first row correctly, but several varint decode assignments used `unknown0, pos, err := ...` style inside the loop. In Go this shadowed the outer `pos`, so the next record resumed from a stale offset.

In the captured two-symbol response, the true second record starts at body offset `81`:

```text
00 30 30 30 30 30 31
```

That is SZ market `0` plus code `000001`. Before the fix, the second decoded quote had a bad market/code because parsing resumed before this offset.

Regression test:

```text
command.TestSecurityQuotesParserKeepsOffsetAcrossRecords
```

Fix:

- decode into `next`
- assign `pos = next` after each successful varint decode
- preserve `Raw` per quote record

## Root Cause: Minute Live Prefix

The live `minute_time` body starts with the usual 4-byte count/header, then a symbol/quote-like prefix:

```text
f0 00 00 00 01 36 30 30 35 31 39 ...
```

The parser previously started minute rows at offset `4`, which treated the prefix as row data. Scanning the fixture showed that offset `65` parses exactly 240 rows to the end of the body with plausible prices and no negative volumes.

Regression test:

```text
command.TestMinuteTimeParserSkipsLiveSymbolPrefix
```

Fix:

- for today minute data only, detect a market byte plus 6 ASCII digits after the 4-byte header
- when present, skip the 65-byte live prefix before parsing rows
- history minute parsing remains unchanged

## Validation Evidence

Focused tests:

```bash
go test -count=1 ./cmd/tdx-probe ./command
```

Full offline verification:

```bash
go test -count=1 ./...
go vet ./...
go test -run=^$ -bench=. -benchmem ./codec ./frame ./command ./validation .
```

Live validation after fixes:

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 45s \
  -operation-timeout 8s \
  -connect-timeout 1s \
  -markets sh \
  -symbols sh:600519 \
  -kline day \
  -skip-boards \
  -skip-files \
  -pretty
```

Result: 12 checks, 10 OK, 2 public-server timeout failures, 0 warnings.

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 80s \
  -operation-timeout 8s \
  -connect-timeout 1s \
  -markets sh,sz \
  -symbols sh:600519,sz:000001 \
  -kline day \
  -skip-boards \
  -skip-files \
  -pretty
```

Result: 14 checks, 12 OK, 2 public-server timeout failures, 0 warnings. `security_quotes` returned both requested symbols.

```bash
TDX_LIVE=1 go run ./cmd/tdx-validate \
  -timeout 70s \
  -operation-timeout 10s \
  -connect-timeout 1s \
  -markets sh \
  -symbols sh:600519 \
  -kline day \
  -pretty
```

Result: 14 checks, 11 OK, 3 failures, 0 warnings. `boards_concept` returned 270 rows. `report_file_base_info.zip` returned 0 bytes on the public server used in this run.

Full security-list validation is now available through `tdx-validate -full-security-list`. A SH-only live run with 35s operation timeout preserved 5000 partial rows before public-server write timeout against a count of 27215. The validator now emits per-page operations before the aggregate full-list result; a follow-up SH smoke showed `security_list_SH_page_0` through `security_list_SH_page_4000` OK and `security_list_SH_page_5000` timing out. Page-level retry is available through `-security-list-page-retries`, and successful retries preserve warning findings for earlier failed attempts. With `-security-list-page-retries 1`, SH and SZ full-list smoke completed 27215 and 23411 rows. BJ count returned 345, but BJ page 0 still timed out with 15s operation timeout and 3 page retries.

## Remaining Work

- Compare the captured fixtures with pytdx/xmtdx output.
- Build a host-operation matrix for report files and history fund flow.
- Implement BJ fallback collection and compare it with `security_count_BJ=345`.
- Add a durable benchmark report artifact after each parser/client change.
