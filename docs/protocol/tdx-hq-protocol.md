# TDX HQ 7709 Protocol Notes

## Frame

Response frame:

```text
0..3    uint32 unknown_0
4..7    uint32 unknown_1
8..11   uint32 unknown_2
12..13  uint16 zipsize
14..15  uint16 unzipsize
16..    body bytes
```

If `zipsize == unzipsize`, body is raw. Otherwise body is zlib-compressed.

The Go implementation exposes the same response through `Client.Capture`: request bytes, the raw 16-byte header, compressed/raw body bytes, decoded body bytes, and the parsed command result are all retained for fixture replay and field audits.

## Connection Setup

After TCP connect, send the three setup byte sequences from `command.SetupCommands`. Their responses are read and discarded.

The client keeps a small per-host idle connection pool by default, so this setup cost is paid once per reusable connection instead of once per request. Connections that return request errors are closed rather than returned to the pool.

## Implemented Requests

- `security_count`: market count request.
- `security_list`: paged security list, 29 bytes per record.
- `security_bars`: stock K-line, price-diff encoded.
- `index_bars`: index K-line, same as stock bars plus up/down counts.
- `security_quotes`: up to 80 symbols per request, includes latest price, OHLC, amount, bid/ask 5 levels, server time, rise speed, and unknown fields retained for audit.
- `minute_time` / `history_minute_time`: cumulative price-diff series; preserves the second unknown varint per row.
- `transaction` / `history_transaction`: cumulative price-diff trade rows; today rows include `NumOrders`, and both today/history preserve trailing unknown varint.
- `finance_info`: latest finance record; stock/share/asset/profit fields are scaled by 10000 where the protocol unit is 万元/万股.
- `xdxr_info`: corporate action and share-change records; record headers are read from the current offset, and share-count fields use the TDX custom volume decoder.
- `history_fund_flow`: category 22 K-line-style request. Response uses a 9-byte prefix, 2-byte count, then 36-byte rows: `YYYYMMDD` plus eight TDX volume-coded amount fields for super/large/medium/small in and out.
- `company_info_category` / `company_info_content`: F10 directory entries and content slices.
- `block_info_meta` / `block_info`: metadata plus chunk download for TDX board files such as `block_gn.dat`.
- `report_file`: chunk download for server files such as `base_info.zip`.

## Operational Notes

A server that completes setup is not necessarily healthy for every operation. Health must be tracked per operation and market. Known unstable cases include SH `security_list` page `start=0` returning empty and BJ list pages timing out on public servers.

The Go client tracks host health both globally and per operation. Consecutive failures can put only that host/operation pair into cooldown, so a node that fails `security_list` can still serve `security_quotes` or report-file requests.

Every request attempt can emit an `Observer` event. The event carries operation, server, attempt number, latency, success/error, parsed row count, capture body size, and whether the connection was reused from the pool. `NewMetricsCollector` can be used directly or as a bridge into OpenTelemetry/logging in the upper application.

Fault behavior is covered by a scriptable local fake server in `tdxtest`. It can replay setup frames and then emit malformed zlib payloads, partial frames, delayed responses, raw bytes, or disconnects, so failover and frame handling can be tested without relying on public server instability.

For protocol reverse engineering, capture live responses with `tdx-probe -capture-dir` or `TDX_LIVE=1 tdx-fixture-matrix`, then compare the parsed result with pytdx/xmtdx JSON using `tdx-compare-py`. This keeps parser changes tied to concrete binary evidence. The matrix tool writes one JSONL status row per operation and continues after individual host/operation failures, which is useful for public-server instability.

`GetMarketStat` and today `GetFundFlow` are canonical client helpers, not standalone verified HQ command payloads: market statistics are derived from SH `880005` quote fields, and today fund flow is calculated from L1 transaction pages using amount thresholds.
