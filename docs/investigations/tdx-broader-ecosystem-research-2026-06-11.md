# TDX Broader Ecosystem Research

Date: 2026-06-11

This document expands the repository roadmap item 7. It deliberately keeps the existing HQ `7709` Go client stable and treats every broader TDX surface as a separate protocol track with its own evidence, fixtures, package boundary, and release gate.

## Sources Reviewed

- pytdx README: standard HQ, extended HQ, reader, financial package, trade-server surfaces: <https://github.com/rainx/pytdx/blob/master/README.md>
- pytdx extended HQ client: <https://github.com/rainx/pytdx/blob/master/pytdx/exhq.py>
- pytdx extended HQ parser directory: <https://github.com/rainx/pytdx/tree/master/pytdx/parser>
- pytdx extended market parser example: <https://github.com/rainx/pytdx/blob/master/pytdx/parser/ex_get_markets.py>
- pytdx extended quote parser example: <https://github.com/rainx/pytdx/blob/master/pytdx/parser/ex_get_instrument_quote.py>
- pytdx local reader directory: <https://github.com/rainx/pytdx/tree/master/pytdx/reader>
- pytdx daily bar reader: <https://github.com/rainx/pytdx/blob/master/pytdx/reader/daily_bar_reader.py>
- pytdx block reader: <https://github.com/rainx/pytdx/blob/master/pytdx/reader/block_reader.py>
- pytdx historical financial crawler: <https://github.com/rainx/pytdx/blob/master/pytdx/crawler/history_financial_crawler.py>
- mootdx quote facade: <https://github.com/mootdx/mootdx/blob/master/mootdx/quotes.py>
- mootdx local reader facade: <https://github.com/mootdx/mootdx/blob/master/mootdx/reader.py>

## Track Ranking

| Rank | Track | Confidence | Why |
|---:|---|---|---|
| 1 | ExHQ `7727` extended market | medium-high | pytdx has a concrete TCP client and parser set for markets, instrument list, quote, bars, minute, and transaction. Live server availability still needs proof. |
| 2 | Local VIPDOC file readers | high | pytdx and mootdx both expose deterministic local file readers. No public server instability. |
| 3 | Professional financial packages | medium-high | pytdx documents the `tdxfin/gpcw*.zip` package flow and parser shape. Our existing report-file command can likely download the same payloads. |
| 4 | Ranking/sorting/market-board discovery | low-medium | Likely exists in TDX ecosystem, but current reviewed pytdx parser list does not show a proven pure TCP implementation beyond current quote/list/bar commands. Needs packet discovery. |
| 5 | Level2 order queue/order detail/depth | low | Requires entitlement and possibly different servers/protocols. No reviewed pure open implementation is strong enough to expose an API. |
| 6 | Trading protocol | low for this repo | pytdx README points to a `TdxTradeServer` wrapper of `trade.dll`, which is not a pure Go TCP data protocol. Treat as a separate repository or explicit opt-in module. |

## Track 1: ExHQ `7727`

Scope:

- Extended markets: futures, options, HK/foreign-style symbols, and other non-standard instruments exposed by TDX ExHQ servers.
- Not a replacement for HQ `7709`; it should live in a new package, likely `exhq`.

Evidence:

- `TdxExHq_API` connects to port `7727` in pytdx examples and sends a different setup command.
- pytdx exposes:
  - `get_markets`
  - `get_instrument_count`
  - `get_instrument_info`
  - `get_instrument_quote`
  - `get_instrument_quote_list`
  - `get_instrument_bars`
  - `get_history_instrument_bars_range`
  - `get_minute_time_data`
  - `get_history_minute_time_data`
  - `get_transaction_data`
  - `get_history_transaction_data`
- pytdx `ex_get_markets.py` shows market rows as 64-byte records with category, GBK name, market id, short name, and unknown bytes.
- pytdx `ex_get_instrument_quote.py` shows quote request bytes starting with `01 01 08 02 02 01 0c 00 0c 00 fa 23`, then `market:uint8 + code[9]`.
- mootdx wraps `TdxExHq_API` in `ExtQuotes` but warns in source that the extended market interface may currently be invalid; this increases live-validation priority.

Proposed public API:

```go
package exhq

func NewClient(opts Options) *Client
func KnownServers() []model.Server

func (c *Client) GetMarkets(ctx context.Context) ([]Market, error)
func (c *Client) GetInstrumentCount(ctx context.Context) (int, error)
func (c *Client) GetInstrumentInfo(ctx context.Context, start, count int) ([]Instrument, error)
func (c *Client) ListInstruments(ctx context.Context, opts ListOptions) (model.PartialResult[Instrument], error)
func (c *Client) GetInstrumentQuote(ctx context.Context, market uint8, code string) (Quote, error)
func (c *Client) GetInstrumentQuoteList(ctx context.Context, market uint8, category uint8, start, count int) ([]Quote, error)
func (c *Client) GetInstrumentBars(ctx context.Context, market uint8, code string, category model.KlineCategory, start, count int) ([]Bar, error)
func (c *Client) GetHistoryInstrumentBarsRange(ctx context.Context, market uint8, code string, start, end int) ([]Bar, error)
func (c *Client) GetMinuteTimeData(ctx context.Context, market uint8, code string) ([]MinuteTime, error)
func (c *Client) GetHistoryMinuteTimeData(ctx context.Context, market uint8, code string, date int) ([]MinuteTime, error)
func (c *Client) GetTransactionData(ctx context.Context, market uint8, code string, start, count int) ([]Transaction, error)
func (c *Client) GetHistoryTransactionData(ctx context.Context, market uint8, code string, date, start, count int) ([]Transaction, error)
```

Boundaries:

- Reuse generic connection-pool/failover ideas, but do not reuse HQ command structs.
- Preserve raw row bytes and unknown fields.
- ExHQ market ids are not `model.MarketSH/SZ/BJ`; use a distinct type.
- No production API stability claim until live probes prove at least markets, count, quote, bars, minute, and transaction on current servers.

## Track 2: Local VIPDOC Readers

Scope:

- Deterministic offline parsing of files under a TDX install directory, such as `vipdoc/sh/lday/sh600000.day`.
- This is not a TCP protocol and should not depend on network transport.

Evidence:

- pytdx `daily_bar_reader.py` parses `.day` rows with struct format `<IIIIIfII`.
- pytdx `block_reader.py` parses block `.dat` files from offset 384, then repeated block name, stock count, block type, and 7-byte codes.
- pytdx reader directory also contains minute, LC minute, extended HQ daily, GB/BQ, and history financial readers.
- mootdx `Reader` facade auto-resolves `vipdoc/{market}/{subdir}/{symbol}.{suffix}` and supports std/ext daily/minute plus block parsing.

Proposed public API:

```go
package vipdoc

type Reader struct { /* root path */ }

func Open(root string) (*Reader, error)
func (r *Reader) Daily(ctx context.Context, symbol model.Symbol) ([]DailyBar, error)
func (r *Reader) Minute(ctx context.Context, symbol model.Symbol, period MinutePeriod) ([]MinuteBar, error)
func (r *Reader) ExtDaily(ctx context.Context, market string, code string) ([]ExtDailyBar, error)
func (r *Reader) ExtMinute(ctx context.Context, market string, code string, period MinutePeriod) ([]ExtMinuteBar, error)
func (r *Reader) BlockFile(ctx context.Context, filename string) ([]BlockMember, error)
func (r *Reader) CustomBlocks(ctx context.Context) ([]CustomBlock, error)
func (r *Reader) GBGQ(ctx context.Context, filename string) ([]CorporateAction, error)
```

Boundaries:

- Accept explicit path from caller; do not guess `C:/new_tdx` unless examples only.
- Return typed parse errors with filename, offset, record size, and expected size.
- Keep parsers streaming-capable for large files.
- No server failover, retries, or heartbeat in this package.

## Track 3: Professional Financial Packages

Scope:

- Historical/professional financial statement packages from `tdxfin/gpcw.txt` and `tdxfin/gpcwYYYYMMDD.zip`.
- This should extend existing report-file capability and provide a dedicated parser.

Evidence:

- pytdx `HistoryFinancialListCrawler` downloads `tdxfin/gpcw.txt` via `get_report_file_by_size`.
- pytdx `HistoryFinancialCrawler` downloads `tdxfin/<filename>`, unzips one `.dat`, reads header `<1hI1H3L`, then stock rows `<6s1c1L`, then float report fields.

Proposed public API:

```go
package financepkg

func ListPackages(ctx context.Context, client ReportFileClient) ([]PackageMeta, error)
func DownloadPackage(ctx context.Context, client ReportFileClient, meta PackageMeta) ([]byte, error)
func ParsePackage(data []byte) (*Package, error)

type Package struct {
    ReportDate int
    FieldCount int
    Records []Record
    Raw []byte
}
```

Boundaries:

- Prefer TDX report-file download; do not bake third-party mirrors into library defaults.
- Keep unknown financial column names as `FieldN` until an audited field dictionary is added.
- Parser tests must use small checked-in synthetic fixtures first; live package fixtures can be large and should be opt-in.

## Track 4: Ranking, Sorting, Auction, Market-Mover Discovery

Scope:

- Market ranking lists, sorted quote pages, auction/异动 data, and specialized market overview commands.

Evidence:

- Current reviewed pytdx parser list does not include clear pure-protocol ranking/sorting parser names.
- Existing HQ quote/list/K-line commands are not enough to infer these safely.

Research path:

1. Add a diagnostic-only `tdx-probe-raw` mode that can send hex request payloads and capture frames without parser registration.
2. Collect request bytes from open-source snippets or packet capture from a user-owned TDX client if legally allowed.
3. For each candidate command, build a command matrix with host, market, payload, zip sizes, body hash, row count guess, and decode notes.
4. Promote to parser only after at least two fixtures decode consistently.

Boundary:

- No public API in root `tdx.Client` until command semantics and field layout are fixture-backed.

## Track 5: Level2 Depth, Order Queue, Order Detail

Scope:

- Ten-level depth, order queue, order-by-order detail, detailed transactions, and other paid Level2 surfaces.

Evidence:

- These are known TDX product capabilities, but the reviewed pure open protocol sources are insufficient for a safe Go API.
- They likely need entitlement, different hosts, or different setup/login behavior.

Research path:

1. Treat as package `level2` with separate `Options`, server list, and entitlement diagnostics.
2. Require user-provided test credentials or a Level2-enabled endpoint before any live probe.
3. Add fake-server and fixture-first parsers before connecting production jobs.
4. Keep every response as `Raw []byte` and expose `UnknownN` until fields are proven.

Boundary:

- Do not add silent fallback from Level2 to L1 quotes. Missing entitlement must be explicit.

## Track 6: Trading Protocol

Scope:

- Login, account, order, cancel, position, trade, and fund queries.

Evidence:

- pytdx README describes trading as using `TdxTradeServer`, a wrapper around `trade.dll`.
- This is not the same as the pure TCP market-data protocol implemented in this repository.

Decision:

- Keep trading out of the `github.com/quantbeing/tdx` market-data client for now.
- If needed, create a separate `tdxtrade` repository/module with explicit legal, credential, and risk controls.

## Recommended Order

1. `exhq`: highest network-protocol value and best open parser evidence.
2. `vipdoc`: deterministic local parser value, low operational risk.
3. `financepkg`: high data value, likely reuses existing report-file transport.
4. ranking/sorting discovery tools: only diagnostic/raw capture first.
5. Level2: only after an entitled endpoint exists.
6. trading: separate repo/module, not part of market-data SDK.

## Acceptance Gate For Any Track

Each track must have all of these before stable API exposure:

- Source evidence linked in docs.
- Command or file format matrix.
- Fixture capture or synthetic fixture.
- Parser unit tests.
- Fake-server or local-file failure tests where applicable.
- Public API doc with partial-result and error behavior.
- Explicit statement of unsupported fields and unknown bytes.
