# Interface Latency Baseline 2026-06-10

## Scope

This records the first interface-level latency baseline after adding:

- `TimeoutPolicy`
- `FastTimeoutPolicy()`
- explicit retry strategies
- `tdx-op-matrix` timeout recommendations

The measurements were run from the current local network on 2026-06-10 Asia/Shanghai.

## Source Runs

### Public API Smoke

Command:

```bash
go run ./cmd/tdx-validate \
  -allow-live \
  -timeout 90s \
  -operation-timeout 2s \
  -connect-timeout 700ms \
  -markets sh,sz,bj \
  -symbols sh:600519,sz:000001 \
  -full-kline \
  -bar-count 10 \
  -transaction-count 50 \
  -history-fund-flow-count 10 \
  > /tmp/tdx-validate-fast-smoke.json
```

Result summary:

| Metric | Value |
|---|---:|
| Duration | `18738ms` |
| Results | `29` |
| OK | `25` |
| Failed | `4` |
| Errors | `4` |
| Warnings | `0` |
| Rows | `53448` |

### Operation/Host Matrix

Command:

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

Result duration: `24629ms`.

## Public API Latency

| Operation | OK | Rows | Latency ms | Notes |
|---|---:|---:|---:|---|
| `security_count_SH` | true | 27234 | 195 | |
| `security_list_SH_0` | true | 1000 | 915 | includes failover/connection cost |
| `security_count_SZ` | true | 23422 | 246 | |
| `security_list_SZ_0` | true | 1000 | 928 | includes failover/connection cost |
| `security_count_BJ` | true | 345 | 842 | count works while BJ list is unstable |
| `security_list_BJ_0` | false | 0 | 2001 | timeout |
| `security_quotes` | true | 2 | 370 | multi-symbol quote |
| `bars_SH_600519_0` | true | 10 | 74 | K-line category 0 |
| `bars_SH_600519_1` | true | 10 | 801 | K-line category 1 |
| `bars_SH_600519_2` | true | 10 | 91 | K-line category 2 |
| `bars_SH_600519_3` | true | 10 | 896 | K-line category 3 |
| `bars_SH_600519_4` | true | 10 | 373 | K-line category 4 |
| `bars_SH_600519_5` | true | 10 | 75 | K-line category 5 |
| `bars_SH_600519_6` | true | 10 | 13 | K-line category 6 |
| `bars_SH_600519_7` | true | 10 | 725 | K-line category 7 |
| `bars_SH_600519_8` | true | 10 | 23 | K-line category 8 |
| `bars_SH_600519_9` | true | 10 | 910 | K-line category 9 |
| `bars_SH_600519_10` | true | 10 | 83 | K-line category 10 |
| `bars_SH_600519_11` | true | 10 | 76 | K-line category 11 |
| `minute_SH_600519` | true | 212 | 15 | likely benefited from warm/reused connection |
| `transaction_SH_600519` | true | 50 | 739 | |
| `market_stat` | true | 1 | 37 | derived from quote `880005` |
| `fund_flow_SH_600519` | true | 1 | 1813 | transaction pagination plus local aggregation |
| `history_fund_flow_SH_600519` | false | 0 | 2001 | fallback hit `history_transaction` timeout |
| `finance_SH_600519` | true | 1 | 891 | |
| `xdxr_SH_600519` | true | 44 | 368 | |
| `company_SH_600519` | true | 16 | 404 | |
| `boards_concept` | false | 0 | 2002 | block file path timeout |
| `report_file_base_info.zip` | false | 0 | 814 | public server returned empty payload |

## Operation/Host Matrix Summary

| Operation | Host | Runs | OK | Failed | Avg ms | Max ms | Last error |
|---|---|---:|---:|---:|---:|---:|---|
| `history-fund-flow` | `115.238.56.198` | 3 | 3 | 0 | 96 | 101 | |
| `history-fund-flow` | `180.153.18.170` | 3 | 3 | 0 | 62 | 63 | |
| `history-fund-flow` | `180.153.18.171` | 3 | 0 | 3 | 701 | 701 | dial timeout |
| `report` | `115.238.56.198` | 3 | 3 | 0 | 127 | 215 | |
| `report` | `180.153.18.170` | 3 | 3 | 0 | 66 | 75 | |
| `report` | `180.153.18.171` | 3 | 0 | 3 | 701 | 701 | dial timeout |
| `security-count` | `115.238.56.198` | 3 | 3 | 0 | 91 | 98 | |
| `security-count` | `180.153.18.170` | 3 | 3 | 0 | 76 | 96 | |
| `security-count` | `180.153.18.171` | 3 | 0 | 3 | 701 | 702 | dial timeout |
| `security-list-bj` | `115.238.56.198` | 3 | 0 | 3 | 2001 | 2001 | read timeout |
| `security-list-bj` | `180.153.18.170` | 3 | 0 | 3 | 2001 | 2001 | read timeout |
| `security-list-bj` | `180.153.18.171` | 3 | 0 | 3 | 700 | 701 | dial timeout |
| `quote` | `115.238.56.198` | 3 | 3 | 0 | 86 | 94 | |
| `quote` | `180.153.18.170` | 3 | 3 | 0 | 88 | 122 | |
| `quote` | `180.153.18.171` | 3 | 0 | 3 | 701 | 701 | dial timeout |

## Interface Categories

### Direct Single-Command APIs

These APIs usually map to one TDX command and are the stable low-latency path:

| API | Command |
|---|---|
| `GetSecurityCount` | `security_count` |
| `GetSecurityList` | `security_list` single page |
| `GetSecurityBars` | `security_bars` |
| `GetIndexBars` | `index_bars` |
| `GetMinuteTimeData` | `minute_time` |
| `GetHistoryMinuteTimeData` | `history_minute_time` |
| `GetTransactionData` | `transaction` |
| `GetHistoryTransactionData` | `history_transaction` |
| `GetFinanceInfo` | `finance_info` |
| `GetXdxrInfo` | `xdxr_info` |
| `GetCompanyInfoCategory` | `company_info_category` |
| `GetCompanyInfoContent` | `company_info_content` |

### Composed APIs

These APIs perform routing, batching, pagination, or chunking:

| API | Composition |
|---|---|
| `Ping` | calls `GetSecurityCount(SH)` |
| `HealthCheck` | executes one or more command objects |
| `GetBars` | routes to stock or index bars by code pattern |
| `GetSnapshot` | alias of `GetSecurityQuotes` |
| `GetSecurityQuotes` | splits symbols into batches of at most `80` |
| `ListSecurities` | per market: count, then `security_list` pages of `1000` |
| `ListAShares` | `ListSecurities(SH,SZ,BJ)` plus A-share code filtering |
| `GetBlockInfo` / `GetBlockInfoWithOptions` | file meta, chunk fetch, then local `.dat` parse; options can cap chunk budget |
| `GetReportFile` / `GetReportFileWithOptions` | repeated `report_file` chunks until short chunk; options can cap chunk budget |
| `ListBoards` / `ListBoardsWithOptions` | maps board type to block file and calls `GetBlockInfo` |
| `ListBoardMembers` / `ListBoardMembersWithOptions` | searches concept/style/index block files |

### Aggregated/Derived APIs

These APIs calculate business models from raw responses:

| API | Aggregation |
|---|---|
| `GetMarketStat` | quote `880005` mapped to up/down/neutral/suspended totals |
| `GetFundFlow` / `GetFundFlowWithOptions` | transaction pagination, de-duplication, and amount-bucket classification; options can cap page budget |
| `GetHistoryFundFlow` / `GetHistoryFundFlowWithOptions` | direct history fund-flow first; if empty, day bars plus historical transactions, then amount-bucket classification; options can cap fallback page budget |

## Current Interpretation

- Healthy-host single command calls generally land in `70-300ms`.
- Public API wrappers that include routing/failover/connection cost commonly land in `300-900ms`.
- Heavy composed or aggregated APIs can approach `1-2s`.
- BJ `security_list` remains operation/market unstable and should use fail-fast plus fallback.
- `180.153.18.171:7709` was a host-level failure in these samples and failed around the configured `700ms` connect timeout.

## Caveats

- `/tmp/tdx-op-matrix-all-fast-goodhosts.json` was discarded because a later all-operation matrix run was interrupted and left a zero-byte file.
- A follow-up fix now keeps `tdx-op-matrix` summary variants separate by `name + operation + host`, preventing SH/SZ/BJ `security_list` rows from being merged under the same operation key.
