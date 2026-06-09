# TDX Go Library Handoff - 2026-06-09

This document preserves the current project context, completed work, remaining work, and operational memory for future RD or agent handoff.

## Current Repository State

- Repository path: `/Users/liuhanqing01/projects/tdx`
- Branch: `main`
- Remote: `git@github-quantbeing:quantbeing/tdx.git`
- Local git identity:
  - `user.name=quantbeing`
  - `user.email=xq.quantbeing@gmail.com`
- Related reference repo config checked from `/Users/liuhanqing01/projects/quantbeing`:
  - remote uses `git@github-quantbeing:quantbeing/quantbeing.git`
  - local identity uses `quantbeing <xq.quantbeing@gmail.com>`
- The previous push error happened because `origin` used default `git@github.com:quantbeing/tdx.git`, which authenticated as GitHub user `lhqlhhh`. The fix was changing origin to the `github-quantbeing` SSH alias.
- Backup branch `backup/before-author-fix` was deleted.
- A filter-branch backup ref may still exist locally at `.git/refs/original/refs/heads/main`. It is not pushed by normal `git push`, but do not run `git push --all` or `git push --mirror` unless this is intentionally cleaned or reviewed.
- As of this handoff, local `main` is ahead of `origin/main` by one commit:
  - `570b50e fix: align kline category wire values`
- Normal push command:

```bash
git push
```

## Project Goal

Build a new Go third-party library for TDX data access. This is not a wrapper around `gotdx`; it directly implements the TDX HQ `7709` TCP private binary protocol.

The library should become a reusable Go package for other projects:

```go
import tdx "github.com/quantbeing/tdx"
```

It should also include diagnostic CLIs and fixture tooling so protocol behavior can be continuously validated against public TDX servers and Python reference implementations.

## Protocol Scope

Current scope is the standard TDX HQ `7709` market data protocol:

- TCP connection setup with three setup frames.
- 16-byte response header.
- zlib body decoding.
- GBK text decoding.
- TDX price varint decoding.
- TDX custom volume float decoding.
- Market data commands for A-share HQ data.

This does not yet mean "all TDX data". The current library covers the HQ `7709` core surface, not the whole TDX ecosystem. The broader ecosystem still includes extension market protocols, Level2, trading, local VIPDOC/professional finance files, ranking lists, option/futures-specific data, and other command families.

## Primary References

- pytdx: `https://github.com/rainx/pytdx`
- xmtdx: `https://pypi.org/project/xmtdx/`
- mootdx: `https://pypi.org/project/mootdx/0.6.9/`
- millken/tdx: `https://pkg.go.dev/github.com/millken/tdx`
- Local prior issue records:
  - `/Users/liuhanqing01/projects/quantbeing/docs/investigations/tdx-go-adapter-issues-2026-06-09.md`
  - `/Users/liuhanqing01/projects/quantbeing/docs/investigations/gotdx-operational-issues-2026-06-06.md`
- Local pytdx scripts:
  - `/Users/liuhanqing01/projects/quantbeing/scripts/tdx`

## Architecture Decisions

- `transport` behavior currently lives inside `client.go` through `Dialer`, `RoundTripper`, and `tcpRoundTripper`.
- `frame` owns response header parsing and zlib/raw body decoding.
- `codec` owns GBK, price, volume, and datetime codecs.
- `command` owns binary request build and response parse for each operation.
- `model` owns public data types and enums.
- `client` exposes stable API methods and handles failover, pooling, health scoring, and canonical helpers.
- All parsed records should preserve `Raw []byte` and unknown fields where practical.
- Static server list is only a seed list. Availability must be measured per operation and per host.
- A host that can complete TCP setup is not necessarily healthy for every operation.
- Single TCP connection requests are serialized. Concurrency should come from multiple hosts and idle pooled connections.
- Quote batch size is capped at `command.MaxQuoteBatch = 80`.
- File chunk size is `DefaultFileChunkSize = 30000`; report file fetch is capped by `MaxFileChunks = 256`.
- Prices exposed in core market models use `model.Decimal` to avoid float price pollution.
- K-line wire categories were corrected on 2026-06-09 to match xmtdx/TDX values:
  - `5m=0`, `15m=1`, `30m=2`, `60m=3`
  - `day=4`, `week=5`, `month=6`
  - `1m=7`, `3m=8`
  - `year=9`, `season=10`, `year_alt=11`

## Completed Work

### Research And Documentation

- Created protocol investigation notes in `docs/investigations/tdx-protocol-python-implementations.md`.
- Created protocol notes in `docs/protocol/tdx-hq-protocol.md`.
- Created capability matrix in `docs/api/capability-matrix.md`.
- Expanded `README.md` with installation, options, public API examples, diagnostics, fault tests, known limits, and agent checklist.

### Protocol Kernel

- Implemented frame header parse and zlib/raw body decode in `frame/frame.go`.
- Implemented codecs:
  - `codec/gbk.go`
  - `codec/price.go`
  - `codec/volume.go`
  - `codec/datetime.go`
- Implemented TDX setup byte sequences in `command/setup.go`.
- Implemented raw capture support through `Client.Capture`.

### Core Commands

Implemented command build/parse for:

- `security_count`
- `security_list`
- `security_bars`
- `index_bars`
- `security_quotes`
- `minute_time`
- `history_minute_time`
- `transaction`
- `history_transaction`
- `finance_info`
- `xdxr_info`
- `history_fund_flow`
- `company_info_category`
- `company_info_content`
- `block_info_meta`
- `block_info`
- `report_file`

Important parser choices:

- Security list keeps raw record bytes and unknown fields.
- Quote parser keeps five-level bid/ask book, server time, rise speed, unknown fields, and raw record bytes.
- Minute-time parser preserves unknown varint fields.
- Transaction parser preserves `NumOrders`, `BuyOrSell`, trailing unknown varint, and raw record bytes.
- XDXR parser reads record headers from current offset and decodes share-count fields with TDX custom volume codec.
- History fund flow category 22 parser is implemented, with client fallback if server returns empty direct response.

### Public Client API

Current public API includes:

- Connection and health:
  - `NewClient(opts Options)`
  - `Close()`
  - `Ping()`
  - `PingAll()`
  - `FromBestHost()`
  - `HealthCheck()`
  - `SetServers()`
  - `ServerStats()`
  - `OperationStats()`
  - `Capture()`
- Securities:
  - `GetSecurityCount()`
  - `GetSecurityList()`
  - `ListSecurities()`
  - `ListAShares()`
  - `ListMarkets()`
- K lines:
  - `GetSecurityBars()`
  - `GetIndexBars()`
  - `GetBars()`
- Quotes:
  - `GetSecurityQuotes()`
  - `GetSnapshot()`
- Minute and transaction data:
  - `GetMinuteTimeData()`
  - `GetHistoryMinuteTimeData()`
  - `GetTransactionData()`
  - `GetHistoryTransactionData()`
- Market statistics and fund flow:
  - `GetMarketStat()`
  - `GetFundFlow()`
  - `GetHistoryFundFlow()`
- Finance and company data:
  - `GetFinanceInfo()`
  - `GetXdxrInfo()`
  - `GetCompanyInfoCategory()`
  - `GetCompanyInfoContent()`
- Boards and files:
  - `GetBlockInfo()`
  - `GetReportFile()`
  - `ListBoards()`
  - `ListBoardMembers()`

### High Availability And Observability

- Implemented per-host idle connection pool.
- Implemented request-level host failover.
- Implemented global host scoring.
- Implemented operation-aware host stats and cooldown.
- Implemented `KeepAliveManager` for repeated heartbeat failure handling.
- Implemented `Observer` hook and `ObserverFunc`.
- Implemented `NewMetricsCollector()` to aggregate per-operation/per-host attempts, successes, failures, latency, row counts, and last error.

### Diagnostics And Test Tools

Implemented CLIs:

- `cmd/tdx-health`
- `cmd/tdx-probe`
- `cmd/tdx-fixture-matrix`
- `cmd/tdx-dump-frame`
- `cmd/tdx-compare-py`

Implemented diagnostics package:

- fixture writing
- default fixture naming
- parsed JSON comparison
- fixture matrix capture

Implemented fake server package:

- `tdxtest.StartScript`
- normal frame responses
- raw responses
- bad zlib responses
- partial frame responses
- delayed responses
- disconnect responses

### Git And Release Preparation

- Initialized the repo and remote binding.
- Set origin to `git@github-quantbeing:quantbeing/tdx.git`.
- Set local git identity to `quantbeing <xq.quantbeing@gmail.com>`.
- Rewrote local commit author/committer identity to match quantbeing identity before pushing.
- Deleted local backup branch `backup/before-author-fix`.
- Current local branch is ready for normal push with `git push`.

## Verified Commands

Fresh verification run before this handoff:

```bash
go test -count=1 ./...
```

Result: all packages passed.

Dry-run push was also checked after the K-line category fix:

```bash
git push --dry-run
```

Result:

```text
To github-quantbeing:quantbeing/tdx.git
   a6ae716..570b50e  main -> main
```

Live TDX smoke tests are not part of default verification. They should be run explicitly with current network conditions.

## Known Limits

- Current library covers HQ `7709` core market data, not every TDX protocol family.
- Public TDX servers are inconsistent by operation and market. A host that succeeds on setup may fail on `security_list`, BJ list, history fund flow, or report files.
- BJ full universe remains partial. The client returns partial result metadata, but robust fallback through `base_info.zip`, securities files, or report-file-derived data still needs implementation.
- `GetMarketStat` is a canonical helper based on SH `880005` quote fields, not a separately verified standalone command.
- Today `GetFundFlow` is derived from transaction aggregation using xmtdx-style amount thresholds.
- History fund flow prefers category 22 direct response; if empty, it falls back to day-bar dates plus historical transaction aggregation.
- Level2 order book,逐笔委托,竞价,异动,排序榜,扩展行情,期货,期权,港股,美股,交易接口,VIPDOC 本地文件解析, and professional finance file parsing are not implemented yet.

## Remaining Work

### P0 - Preserve And Publish Current State

1. Push current local branch:

```bash
git push
```

2. Optionally remove old filter-branch backup ref after confirming no rollback is needed:

```bash
git update-ref -d refs/original/refs/heads/main
```

Do not remove it if you still want a local-only recovery point for pre-author-rewrite history.

### P1 - Live Protocol Fixture Collection

Run current HQ operation matrix against live public servers:

```bash
TDX_LIVE=1 go run ./cmd/tdx-fixture-matrix \
  -out ./fixtures/live/2026-06-09 \
  -ops security-count,security-list-sh,security-list-sz,security-list-bj,stock-bars,index-bars,quote,market-stat-source,minute,transaction,finance,xdxr,company,block-meta,block,report,history-fund-flow
```

If the exact operation names differ, inspect `diagnostic.DefaultMatrixOperations()` and `cmd/tdx-probe commandFor()` before running.

Expected outcome:

- JSONL matrix rows for every attempted operation.
- Fixture files containing request bytes, header bytes, raw body, decoded body, and parsed JSON.
- A server-operation failure matrix that identifies hosts unsuitable for specific commands.

### P1 - Python Reference Comparison

Use `/Users/liuhanqing01/projects/quantbeing/scripts/tdx` and pytdx/xmtdx scripts to capture reference JSON for the same symbols and dates.

Then compare with:

```bash
go run ./cmd/tdx-compare-py \
  -go ./fixtures/live/<go.fixture.json> \
  -py ./fixtures/pytdx/<py.json> \
  -max-diffs 100 \
  -tolerance 0.0001
```

Expected outcome:

- Parser mismatches classified into real bugs, known unknown fields, or acceptable numeric tolerance differences.
- New binary fixtures committed for every parser fix.

### P1 - BJ Universe Fallback

Implement a stable BJ list fallback instead of relying only on public-server `security_list` behavior.

Suggested route:

1. Fetch `base_info.zip` or other securities/report files through `GetReportFile`.
2. Decode zip/file structure.
3. Extract securities into `model.Security` or a dedicated fallback model.
4. Merge with direct `security_list` partial result.
5. Preserve failure metadata in `model.PartialResult`.

Files likely to change:

- `client.go`
- `command/company_files.go`
- `model/types.go`
- new parser package or file for report/base-info decoding
- `client_test.go`
- `tdxtest/fakeserver.go`

### P1 - Operation-Aware Live Health

Current `HealthCheck` accepts command objects and `OperationStats` tracks per-operation state. The next useful addition is a richer live health matrix command.

Suggested output fields:

- host
- operation
- market
- success
- latency
- decoded body length
- parsed row count
- error
- cooldown status

This can extend `tdx-health` or become `tdx-health -matrix`.

### P2 - API Stability And Versioning

Before tagging v0.1.0:

- Review public structs for JSON tags and field names.
- Decide whether all `float64` volume/amount fields should move to fixed-point or decimal wrappers.
- Add package-level documentation.
- Add `CHANGELOG.md`.
- Tag release only after live smoke and core fixture comparison.

### P2 - Extended TDX Surfaces

Research and implement more command families after HQ core is stable:

- extension market data
- ranking/sort APIs
- sector statistics
- auction data
- abnormal movement data
- local VIPDOC file parsing
- professional finance file parsing
- Level2 depth and order data if protocol/source access is available
- futures/options/HK/US surfaces if they are served by reachable protocols

Each new surface should follow the same pattern:

1. Capture raw request/response from Python or live probe.
2. Add binary fixture.
3. Write parser test first.
4. Implement command build/parse.
5. Expose raw command API if needed.
6. Add canonical client helper only when behavior is stable.
7. Update README and capability matrix.

## Safe Continuation Checklist

When continuing from a fresh session:

1. Open this file first:

```bash
sed -n '1,260p' docs/handoffs/tdx-go-library-handoff-2026-06-09.md
```

2. Check repo state:

```bash
git status --short --branch
git remote -v
git log --oneline --decorate --max-count=12
```

3. Run verification:

```bash
go test -count=1 ./...
```

4. Read current API docs:

```bash
sed -n '1,260p' README.md
sed -n '1,220p' docs/api/capability-matrix.md
sed -n '1,220p' docs/protocol/tdx-hq-protocol.md
```

5. For protocol work, capture before changing parsers:

```bash
go run ./cmd/tdx-probe -op quote -capture-dir ./fixtures/live
```

6. For network or parser bugs, reproduce with `tdxtest.StartScript` or a committed fixture before changing code.

## Commit History Summary

Current mainline commits:

```text
5159db5 feat: add protocol codecs and models
7c557b7 feat: implement tdx command parsers
d5d3795 feat: add client failover and pooling
f75e0ea feat: add diagnostics and fixture tools
04a29d2 docs: document tdx protocol implementation
5b2e1e1 feat: add request observer metrics
30b5bc8 test: add fake tdx fault server
a6ae716 docs: expand api usage readme
570b50e fix: align kline category wire values
```

All current mainline commits should be authored and committed as:

```text
quantbeing <xq.quantbeing@gmail.com>
```

## Human Context

The user wants a production-useful Go third-party library, not an academic protocol note and not a gotdx wrapper. The library should be rich enough that other RD or agents can read `README.md` and use the API smoothly.

The user explicitly asked to:

- reference the most complete TDX interface surfaces available
- study Python protocol implementations, especially pytdx and xmtdx
- borrow xmtdx ideas including heartbeat/keepalive and pytdx bug fixes
- preserve availability and efficiency
- support high availability through failover, health scoring, and per-operation awareness
- keep raw bytes and unknown fields for future reverse engineering
- document usage clearly
- keep local git identity consistent with quantbeing
- bind the repo to `git@github.com:quantbeing/tdx.git` through the quantbeing SSH alias

The strategic answer remains:

- Current v0 is suitable as a reusable Go HQ `7709` core market-data library.
- It is not yet a complete SDK for every possible TDX data source.
- The next highest-value work is live fixture capture, Python comparison, and BJ/report-file fallback.
