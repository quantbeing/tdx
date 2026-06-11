# TDX Protocol Interface Map

This document maps the public Go API to the underlying TDX HQ TCP protocol commands implemented by this repository. It is intended as the pre-tag interface/protocol reference for RD and agent consumers.

Current scope: HQ TCP `7709` market-data protocol. This is not the complete TDX ecosystem; Level2, trading, futures/options, local VIPDOC, and professional finance-package protocols are still outside the stable client surface.

## Transport And Frame Model

### TCP Session

The client connects to a TDX HQ server over TCP, usually port `7709`.

Implementation path:

- `NewClient(opts Options)` configures servers, timeouts, retries, pooling, circuit breaker, and observer hooks.
- `NetDialer.DialTDX(ctx, server, opts)` opens `tcp` to `server.Addr()`.
- `tcpRoundTripper.setup()` writes the three setup frames from `command.SetupCommands`.
- Each setup request expects one normal TDX response frame.
- After setup, each request is serialized on a single TCP connection by `tcpRoundTripper.mu`; concurrency comes from host failover and pooled independent connections, not concurrent writes on one connection.

Setup frames:

| Step | Hex request |
|---:|---|
| 1 | `0c0218930001030003000d0001` |
| 2 | `0c0218940001030003000d0002` |
| 3 | `0c031899000120002000db0fd5d0c9ccd6a4a8af0000008fc22540130000d500c9ccbdf0d7ea00000002` |

### Request/Response Flow

Every protocol command implements:

```go
type command.Command interface {
    BuildRequest() ([]byte, error)
    ParseResponse(body []byte) (any, error)
    Operation() string
}
```

`tcpRoundTripper.RoundTripRaw`:

1. Calls `cmd.BuildRequest()`.
2. Writes raw request bytes to TCP.
3. Reads 16-byte response header.
4. Reads `Header.ZipSize` bytes of raw body.
5. If `ZipSize != UnzipSize`, zlib-decompresses the body.
6. Calls `cmd.ParseResponse(decodedBody)`.
7. Returns `CapturedResponse` with request bytes, header bytes, compressed raw body, decoded body, parsed value, server, attempt, and latency.

`tcpRoundTripper.RoundTrip` returns only `CapturedResponse.Parsed`.

### Diagnostic Raw Probe

`tdx-probe -raw-hex <hex> -capture-dir <dir>` can send a caller-supplied raw request payload and preserve the response as a fixture with operation `raw_probe`.

This mode is diagnostic-only. It is intended for legally permitted protocol research on owned or explicitly authorized endpoints. It bypasses typed command builders, returns decoded body bytes without field parsing, and is not part of the stable public API.

### Response Header

TDX responses use a fixed 16-byte little-endian header:

| Offset | Field | Type | Meaning |
|---:|---|---|---|
| 0 | `Unknown0` | `uint32` | Preserved; not interpreted yet. |
| 4 | `Unknown1` | `uint32` | Preserved; not interpreted yet. |
| 8 | `Unknown2` | `uint32` | Preserved; not interpreted yet. |
| 12 | `ZipSize` | `uint16` | Bytes to read after header. |
| 14 | `UnzipSize` | `uint16` | Expected decoded body length. If equal to `ZipSize`, body is raw. |

The frame decoder validates both compressed body length and decoded body length.

## Common Field Codecs

| Codec | Implementation | Used By | Rule |
|---|---|---|---|
| TDX price varint | `codec.GetPrice` | quote, bars, minute, transaction | First byte stores 6 value bits, sign bit `0x40`, continuation bit `0x80`; following bytes store 7-bit continuation chunks. |
| TDX volume float | `codec.GetVolume`, `codec.DecodeVolume` | bars amount/volume, quote amount, finance share changes | Decodes TDX custom 4-byte floating representation using exponent-like high byte and mantissa-like low bytes. |
| GBK string | `codec.DecodeGBKBestEffort` | security name, company files, board files | Trim NUL, GBK decode, fallback to valid UTF-8 replacement, trim spaces. |
| K-line datetime | `codec.GetDateTime` | bars, xdxr | Intraday categories use packed `zipday + minutes`; daily or higher categories use `YYYYMMDD`. |
| Transaction time | `codec.GetTime` | transactions | `uint16` minutes from midnight. |
| Fixed decimal | `model.Decimal` | prices and rise speed | Stores mantissa and scale to avoid float pollution. Quote prices use scale `2`; K-line prices use milli scale `3`. |

All row-oriented parsers keep `Raw []byte` for future protocol reverse engineering. Unknown fields are exposed as `UnknownN` when their position is known but meaning is not stable.

## Raw Protocol Command Map

### Security Count

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetSecurityCount(ctx, market)` | `command.NewSecurityCountCommand(market)` | `security_count` | Prefix `0c0c186c0001080008004e04`, then market patched into trailing payload. |

Response parser:

- Requires at least 2 bytes.
- Reads `uint16` little-endian count.

Return type: `uint16`.

### Security List

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetSecurityList(ctx, market, start)` | `command.NewSecurityListCommand(market, start)` | `security_list` | Prefix `0c0118640101060006005004`, then `market:uint16`, `start:uint16`, trailing `0:uint16`. |

Response parser:

- Header: first 2 bytes = row count.
- Each row is 29 bytes.

Per-row fields:

| Bytes | Field | Decode |
|---:|---|---|
| 0..6 | `Code` | ASCII, trim NUL/space. |
| 6..8 | `VolUnit` | `uint16` little-endian. |
| 8..16 | `Name` | GBK fixed string. |
| 16..20 | `Unknown1` | `[4]byte`, preserved. |
| 20 | `DecimalPoint` | raw byte. |
| 21..25 | `PreClose` | TDX volume codec. |
| 25..29 | `Unknown2` | `[4]byte`, preserved. |
| all | `Raw` | original 29-byte record. |

Return type: `[]model.Security`.

Known behavior:

- Public BJ `security_list` is unstable in current live tests. `ListAShares()` defaults to SH/SZ; BJ is explicit opt-in through `ListASharesWithOptions`.

### K Lines: Stock And Index

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetSecurityBars(ctx, market, code, category, start, count)` | `command.NewSecurityBarsCommand(...)` | `security_bars` | Generic bars request with command category `0x052d`, market, code, kline category, start, count. |
| `GetIndexBars(ctx, market, code, category, start, count)` | `command.NewIndexBarsCommand(...)` | `index_bars` | Same wire format as stock bars, parser additionally reads index breadth fields. |

K-line categories are listed from smallest duration to largest. The numeric value is the TDX wire category and intentionally does not follow duration order.

| Category | Value |
|---|---:|
| `KlineMinute1` | 7 |
| `KlineMinute3` | 8 |
| `KlineMinute5` | 0 |
| `KlineMinute15` | 1 |
| `KlineMinute30` | 2 |
| `KlineMinute60` | 3 |
| `KlineDay` | 4 |
| `KlineWeek` | 5 |
| `KlineMonth` | 6 |
| `KlineSeason` | 10 |
| `KlineYear` | 9 |
| `KlineYearAlt` | 11 |

Response parser:

- Header: first 2 bytes = row count.
- Each row is variable length because price and volume values use codecs.
- Datetime decoded by kline category.
- `openDiff`, `closeDiff`, `highDiff`, `lowDiff` are TDX price varints.
- OHLC absolute values are reconstructed by a rolling previous base:
  - `openAbs = openDiff + preDiffBase`
  - `closeAbs = openAbs + closeDiff`
  - `highAbs = openAbs + highDiff`
  - `lowAbs = openAbs + lowDiff`
  - `preDiffBase = openAbs + closeDiff`
- `Vol` and `Amount` use TDX volume codec.
- Index rows read extra `UpCount:uint16` and `DownCount:uint16`.
- `Raw` preserves row bytes.

Return type: `[]model.Bar`.

### Quotes And Snapshot

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetSecurityQuotes(ctx, symbols)` | `command.NewSecurityQuotesCommand(symbols)` | `security_quotes` | Header starts with `0x010c`, command family `0x02006320`, category `0x0005053e`, then symbol count and repeated `market:byte + code[6]`. |
| `GetSnapshot(ctx, symbols)` | same | `security_quotes` | Alias of `GetSecurityQuotes`. |

Limits:

- One protocol request supports at most `command.MaxQuoteBatch = 80` symbols.
- `GetSecurityQuotes` automatically splits larger input into batches of 80 and appends results.

Response parser:

- First 2 bytes currently skipped.
- Next 2 bytes = row count.
- Per row:
  - `market:byte`
  - `code[6]`
  - `Active1:uint16`
  - `Price` price varint, decimal scale 2.
  - `PreClose`, `Open`, `High`, `Low` are decoded as deltas from `Price`.
  - `Unknown0`, `Unknown1`, `Vol`, `CurVol`, `SVol`, `BVol`, `Unknown2`, `Unknown3`, `Unknown5`..`Unknown8` use price-varint positions.
  - `Amount` uses TDX volume codec.
  - Five bid/ask levels are decoded as price deltas from `Price` plus bid/ask volumes.
  - `Unknown4:uint16`
  - `RiseSpeed:int16`, decimal scale 2.
  - `Active2:uint16`
  - `ServerTime` is derived from `Unknown0` by `FormatServerTime`.
  - `Raw` preserves row bytes.

Return type: `[]model.Quote`.

### Minute Time

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetMinuteTimeData(ctx, market, code)` | `command.NewMinuteTimeDataCommand(...)` | `minute_time` | Prefix `0c1b080001010e000e001d05`, then `market:uint16`, `code[6]`, trailing `0:uint32`. |
| `GetHistoryMinuteTimeData(ctx, market, code, date)` | `command.NewHistoryMinuteTimeDataCommand(...)` | `history_minute_time` | Prefix `0c01300001010d000d00b40f`, then `date:uint32`, `market:byte`, `code[6]`. |

Response parser:

- Count is `uint16` at body start.
- Today minute data normally skips 4 bytes. If an extended symbol prefix is detected, it skips 65 bytes.
- History minute data skips 6 bytes.
- Each row:
  - `priceDiff` via price varint; accumulated into last price.
  - `Unknown1` via price varint.
  - `Volume` via price varint.
  - Price is decimal scale 2.
  - `Raw` preserves row bytes.

Return type: `[]model.MinuteTime`.

### Transaction Data

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetTransactionData(ctx, market, code, start, count)` | `command.NewTransactionDataCommand(...)` | `transaction` | Prefix `0c17080101010e000e00c50f`, then `market:uint16`, `code[6]`, `start:uint16`, `count:uint16`. |
| `GetHistoryTransactionData(ctx, market, code, date, start, count)` | `command.NewHistoryTransactionDataCommand(...)` | `history_transaction` | Prefix `0c013001000112001200b50f`, then `date:uint32`, `market:uint16`, `code[6]`, `start:uint16`, `count:uint16`. |

Response parser:

- Count is `uint16` at body start.
- History transaction starts rows at offset 6; today transaction starts at offset 2.
- Per row:
  - `Hour`, `Minute` from `uint16` minutes from midnight.
  - `priceDiff` via price varint; accumulated into last price.
  - `Vol` via price varint.
  - `NumOrders` via price varint only for today transaction.
  - `BuyOrSell` via price varint.
  - `UnknownLast` via price varint.
  - `Raw` preserves row bytes.

Return type: `[]model.Transaction`.

### History Fund Flow

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetHistoryFundFlow(ctx, market, code, start, count)` | `command.NewHistoryFundFlowCommand(...)` first | `history_fund_flow` | Bars-like command family `0x052d` with category `22`, market, code, start, count. |

Direct response parser:

- If body shorter than 11 bytes, returns empty `[]model.HistoricalFundFlow`.
- Row count at `body[9:11]`.
- Each row:
  - `rawDate:uint32` as `YYYYMMDD`.
  - 8 amount fields via TDX volume codec:
    - `SuperIn`
    - `LargeIn`
    - `MediumIn`
    - `SmallIn`
    - `SuperOut`
    - `LargeOut`
    - `MediumOut`
    - `SmallOut`
  - `Raw` preserves row bytes.

Canonical fallback behavior is described in the composite API section.

### Finance Info

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetFinanceInfo(ctx, market, code)` | `command.NewFinanceInfoCommand(...)` | `finance_info` | Prefix `0c1f187600010b000b0010000100`, then `market:byte`, `code[6]`. |

Response parser:

- Requires `2 + 7 + financeInfoSize` bytes.
- Skips 2-byte header.
- Reads response `market:byte + code[6]`.
- Finance block:
  - `LiutongGuben:float32`
  - `Province:uint16`
  - `Industry:uint16`
  - `UpdatedDate:uint32`
  - `IPODate:uint32`
  - 30 `float32` fields.
- Most financial amount/share fields are scaled by `*10000`.
- `Raw` preserves the finance block.

Return type: `model.FinanceInfo`.

### XDXR Info

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetXdxrInfo(ctx, market, code)` | `command.NewXdxrInfoCommand(...)` | `xdxr_info` | Prefix `0c1f187600010b000b000f000100`, then `market:byte`, `code[6]`. |

Response parser:

- If body shorter than 11 bytes, returns empty slice.
- Count at offset 9.
- Per row:
  - `market:byte + code[6]`.
  - 1 skipped byte.
  - date via daily `codec.GetDateTime`.
  - `Category:byte`.
  - 16-byte category payload.
- Category-specific payload:
  - Category `1`: cash dividend, allotment price, bonus/transfer shares, allotment shares from float32 values.
  - Category `11`, `12`: `Suogu` from float32.
  - Category `13`, `14`: exercise price and shares from float32.
  - Other categories: before/after float shares and total shares via TDX volume codec.
- `Name` is mapped from known category code.
- `Raw` preserves row bytes.

Return type: `[]model.XdxrRecord`.

### Company Info Category And Content

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetCompanyInfoCategory(ctx, market, code)` | `command.NewCompanyInfoCategoryCommand(...)` | `company_info_category` | Prefix `0c0f109b00010e000e00cf02`, then `market:uint16`, `code[6]`, trailing `0:uint32`. |
| `GetCompanyInfoContent(ctx, market, code, filename, offset, length)` | `command.NewCompanyInfoContentCommand(...)` | `company_info_content` | Prefix `0c07109c000168006800d002`, then market, code, filename[80], offset, length. |

Category parser:

- First 2 bytes = count.
- Each record is 152 bytes:
  - `Name`: bytes `0..64`, GBK.
  - `Filename`: bytes `64..144`, GBK.
  - `Start:uint32`.
  - `Length:uint32`.
  - `Raw`.

Content parser:

- Requires at least 12 bytes.
- Length is `uint16` at offset `10..12`.
- Returns raw content bytes from offset 12 with requested length validation.

Return types:

- `[]model.CompanyInfoCategory`
- `[]byte`

### Block Info And Report File

| Public API | Command | Operation | Request |
|---|---|---|---|
| `GetBlockInfo(ctx, filename)` | `BlockInfoMetaCommand` + repeated `FileChunkCommand` | `block_info_meta`, `block_info` | Metadata prefix `0c39186900012a002a00c502`; chunk prefix `0c37186a00016e006e00b906`. |
| `GetReportFile(ctx, filename)` | repeated `FileChunkCommand` | `report_file` | Same chunk wire prefix as block chunk, operation name differs in parser context. |

Block metadata parser:

- Requires at least 38 bytes.
- `Size:uint32` at offset 0.
- `Hash` from bytes `5..37`, trim NUL/space.

Chunk parser:

- If body shorter than 4 bytes, returns empty bytes.
- Otherwise returns `body[4:]`.

Block `.dat` parser:

- Requires at least 386 bytes.
- Board count at offset 384.
- Each board record is 2813 bytes:
  - `Name`: bytes `0..9`, GBK.
  - `Count:uint16`: bytes `9..11`.
  - `boardType:uint16`: bytes `11..13`.
  - Up to 400 member codes; each code is 7 bytes starting at offset 13.
  - Category inferred from filename (`gn`, `fg`, `zs`) unless record type overrides it.
  - `Raw` preserves board row bytes.

Chunk budgets:

- Default chunk size: `DefaultFileChunkSize = 30000`.
- Default max chunks: `MaxFileChunks = 256`.
- `FileFetchOptions` can lower chunk count or chunk size.
- `IsChunkBudgetError(err)` classifies budget exhaustion.

## Composite And Canonical API Map

These APIs are not one-to-one wire commands. They combine raw commands, route by code, page results, aggregate rows, or apply failure semantics.

| Public API | Composition | Error/Partial Semantics |
|---|---|---|
| `Ping(ctx)` | Calls `GetSecurityCount(ctx, MarketSH)`. | Returns raw error from `security_count`. |
| `PingAll(ctx, servers, transport)` | For each server, runs TCP/setup only. | Only proves setup reachability, not operation health. |
| `FromBestHost(ctx, opts)` | Calls `PingAll`, selects lowest setup latency host. | Operation-specific failures can still happen later. |
| `FromBestHostByOperations(ctx, opts, probes...)` | For each server, creates single-host client and runs `HealthCheck` with caller-provided commands. | Returns selected client plus `[]HostHealth` evidence; fails if no host passes all probes. |
| `HealthCheck(ctx, ops...)` | Executes each command through normal `Client.execute`. | Records operation, OK, latency, error. |
| `Capture(ctx, cmd)` | Executes one raw command and returns `CapturedResponse`. | Preserves request/header/raw compressed body/decoded body/parsed result. |
| `WithRequestOptions(ctx, opts)` | Not a TDX command. The context carries request policy for subsequent API calls. | `MaxAttempts` and `Retry` override the client policy for this request chain. `TimeoutPolicy` overrides only non-zero operation/market/default entries; unspecified entries inherit the client policy. |
| `ListSecurities(ctx, markets...)` | For each market: `GetSecurityCount`, then `GetSecurityList` pages of 1000. | Returns `model.PartialResult[Security]`; failures are in `Failures`; `IsPartialResultError(err)` means items may still be usable. |
| `ListSecuritiesWithOptions(ctx, opts)` | Same as `ListSecurities`, with `Markets`, `MaxPagesPerMarket`, `StopOnError`. | Page budget creates failure operation `security_list_budget`. |
| `ListAShares(ctx)` | Calls `ListASharesWithOptions` with default markets SH/SZ, then filters A-share code prefixes. | BJ is intentionally not default because public BJ list is unstable. |
| `ListASharesWithOptions(ctx, opts)` | Calls `ListSecuritiesWithOptions`, then filters A-share prefixes. | If caller includes BJ, BJ failures are partial result failures. |
| `ListMarkets(ctx)` | Returns `[SH,SZ,BJ]`. | No network call. |
| `GetBars(ctx, market, code, category, start, count)` | Routes to `GetIndexBars` if code looks index-like, otherwise `GetSecurityBars`. | Index-like prefixes: SH `000/880/881/882/883/884/885/999`, SZ `395/399`. |
| `GetSnapshot(ctx, symbols)` | Alias of `GetSecurityQuotes`. | Uses quote batch splitting. |
| `GetMarketStat(ctx)` | Calls `GetSecurityQuotes` for SH `880005`. | Maps quote fields into up/down/neutral/total; suspended is residual `total-up-down-neutral`. |
| `GetFundFlow(ctx, market, code)` | Calls `GetFundFlowWithOptions` with defaults. | Derived, not direct protocol command. |
| `GetFundFlowWithOptions(ctx, market, code, opts)` | Pages today `GetTransactionData`, deduplicates repeated records/pages, classifies by amount bucket. | `IsPageBudgetError(err)` on page budget exhaustion. |
| `GetHistoryFundFlow(ctx, market, code, start, count)` | Calls `GetHistoryFundFlowWithOptions` with defaults. | Direct command first, fallback if direct response empty. |
| `GetHistoryFundFlowWithOptions(ctx, market, code, start, count, opts)` | First executes `HistoryFundFlowCommand` category 22. If rows are empty, calls `GetSecurityBars(day)` to obtain dates, then pages `GetHistoryTransactionData` per date and classifies. | Context errors abort. Non-context direct command errors fall through to fallback. Page budget applies to fallback transaction paging. |
| `GetBlockInfo(ctx, filename)` | Calls `GetBlockInfoWithOptions` with default chunk budget. | Metadata, chunks, local `.dat` parser. |
| `GetBlockInfoWithOptions(ctx, filename, opts)` | `BlockInfoMetaCommand`, then repeated `NewBlockInfoCommand(filename,start,length)`, then `ParseBlockData`. | Empty metadata/payload and invalid board payload return explicit errors; chunk budget errors are classifiable. |
| `GetReportFile(ctx, filename)` | Calls `GetReportFileWithOptions` with default chunk budget. | Returns raw file bytes. |
| `GetReportFileWithOptions(ctx, filename, opts)` | Repeated `NewReportFileCommand(filename,start,length)` until short chunk. | Empty payload and chunk-budget exhaustion return explicit errors. |
| `ListBoards(ctx, boardType)` | Maps type to block file then calls `GetBlockInfo`. | `concept -> block_gn.dat`, `style -> block_fg.dat`, `industry/index -> block_zs.dat`. |
| `ListBoardsWithOptions(ctx, boardType, opts)` | Same as `ListBoards`, with chunk budget. | Chunk budget is classifiable. |
| `ListBoardMembers(ctx, boardCode)` | Calls `ListBoardMembersWithOptions`. | Searches concept/style/index block files. |
| `ListBoardMembersWithOptions(ctx, boardCode, opts)` | Tries `block_gn.dat`, `block_fg.dat`, `block_zs.dat` through `GetBlockInfoWithOptions`, then matches board name and returns codes. | Context and chunk-budget errors return immediately; other block-file errors are skipped. |

### A-share Filter Rules

`ListAShares` and `ListASharesWithOptions` use code-prefix filtering:

| Market | Prefixes |
|---|---|
| SH | `600`, `601`, `603`, `605`, `688`, `689` |
| SZ | `000`, `001`, `002`, `003`, `300`, `301` |
| BJ | `4`, `8`, `920` |

### Fund Flow Classification

Today fund flow and fallback history fund flow classify transactions by `amount = price * volume * 100`:

| Amount | Bucket |
|---:|---|
| `> 1,000,000` | Super |
| `> 200,000` | Large |
| `> 40,000` | Medium |
| otherwise | Small |

`BuyOrSell == 0` is treated as inflow. `BuyOrSell == 1` is treated as outflow. Other values are ignored.

Pagination and deduplication:

- Transaction pages are fetched with default page size 2000 for today fund flow and 800 for history fallback.
- Default max start is 10000.
- Duplicate records are removed by `(hour, minute, price, volume, buy/sell, unknownLast)`.
- Repeated pages are detected by first/last transaction signatures.
- Paging stops on empty page, duplicate page, no new records, or a page shorter than 100 rows.

## High Availability And Execution Semantics

`Client.execute` wraps every command with:

- Request-level host failover.
- Per-operation host stats.
- Operation-aware circuit breaker and cooldown.
- Per-host idle connection reuse.
- Retry strategy:
  - Default `RetryStrategyFailoverFirst`: failed attempt switches host.
  - Optional `RetryStrategySameHostFirst`: spends configured attempts on same host first.
- Timeout policy:
  - Global/default timeout.
  - Per-operation timeout.
  - Per-market/per-operation timeout, e.g. BJ `security_list`.
- Observer events with operation, server, attempt, latency, rows, error, and reused connection flag.

The transport discards failed connections and returns successful connections to the per-host idle pool.

## Error Classification

| Helper | Meaning |
|---|---|
| `IsPartialResultError(err)` | A typed partial result is returned and `Items` may be usable. Inspect `Failures`. |
| `IsBudgetError(err)` | Any explicit caller budget was exceeded. |
| `IsChunkBudgetError(err)` | File/block chunk budget exceeded. |
| `IsPageBudgetError(err)` | Transaction pagination budget exceeded. |

## Important Known Limits

- `security_count_BJ` can work while `security_list_BJ_page_0` times out on public servers.
- `ListAShares()` therefore defaults to SH/SZ. Include BJ only when the caller can tolerate partial results or has a fallback.
- `GetMarketStat` is derived from quote `880005`, not a separately verified market-stat command.
- Today `GetFundFlow` is derived from transaction aggregation, not a direct HQ command.
- `GetHistoryFundFlow` direct category 22 can return empty; fallback recomputes from daily bars and historical transaction pages.
- Company content, report files, and board files are raw/protocol-file surfaces. Some public servers can return empty payloads for files such as `base_info.zip`.
