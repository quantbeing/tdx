# tdx

`github.com/quantbeing/tdx` 是一个全新的 Go 版通达信 HQ `7709` TCP 私有协议库。

它不是 gotdx wrapper。当前实现直接处理通达信 TCP 连接、三段 setup、16 字节响应 header、zlib 解压、GBK 文本、价格 varint、TDX 自定义 volume float，以及各类命令的二进制 build/parse。

这个仓库可以作为两类东西使用：

- Go 第三方库：业务系统直接 `import "github.com/quantbeing/tdx"` 调用行情、K 线、快照、分时、逐笔、财务、板块、报表等接口。
- 协议诊断工具：用 `tdx-health`、`tdx-probe`、`tdx-data-probe`、`tdx-fixture-matrix`、`tdx-dump-frame`、`tdx-compare-py` 做公网节点探测、官方数据包 fallback 探测、raw fixture 抓取、Python 对照和协议反推。

当前仍是 v0 阶段，接口已经按稳定 API 方向组织，但通达信公网服务器和非官方协议本身存在不稳定性。生产接入时请保留超时、重试、host failover、fixture 对照和运行指标。

## Install

仓库 push 到 GitHub 后，其他项目可以直接使用：

```bash
go get github.com/quantbeing/tdx
```

本地联调时可以在业务项目的 `go.mod` 里临时加：

```go
replace github.com/quantbeing/tdx => /Users/liuhanqing01/projects/tdx
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    tdx "github.com/quantbeing/tdx"
    "github.com/quantbeing/tdx/model"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()

    client := tdx.NewClient(tdx.Options{})
    defer client.Close()

    count, err := client.GetSecurityCount(ctx, model.MarketSH)
    if err != nil {
        panic(err)
    }
    fmt.Println("SH security count:", count)
}
```

默认配置会使用内置通达信公网服务器种子、每 host 1 条 idle 连接、请求级 host failover、operation-aware 熔断冷却。

## Client Options

```go
metrics := tdx.NewMetricsCollector()

client := tdx.NewClient(tdx.Options{
    Servers: []model.Server{
        {Name: "tdx-sh-170", Host: "180.153.18.170", Port: 7709},
        {Name: "tdx-sh-171", Host: "180.153.18.171", Port: 7709},
    },
    Timeout:     8 * time.Second,
    MaxAttempts: 2,
    Transport: tdx.TransportOptions{
        ConnectTimeout: 800 * time.Millisecond,
        ReadTimeout:    8 * time.Second,
        WriteTimeout:   800 * time.Millisecond,
    },
    TimeoutPolicy: tdx.FastTimeoutPolicy(),
    Retry: tdx.RetryOptions{
        Strategy: tdx.RetryStrategyFailoverFirst,
    },
    Pool: tdx.PoolOptions{
        MaxIdlePerHost: 2,
    },
    CircuitBreaker: tdx.CircuitBreakerOptions{
        FailureThreshold: 2,
        Cooldown:         30 * time.Second,
    },
    Observer: metrics,
})
defer client.Close()
```

字段说明：

| Option | 用途 |
|---|---|
| `Servers` | 自定义通达信服务器列表；为空时使用 `KnownServers()`。 |
| `Timeout` | 默认总超时，同时会填充未设置的 connect/read/write timeout。 |
| `MaxAttempts` | 单次请求最多 attempt 次数；默认等于 server 数量。设置得大于 server 数量时，failover-first 会轮询完一轮后继续下一轮。 |
| `Transport` | TCP connect/read/write timeout。 |
| `TimeoutPolicy` | 按 operation/market 给 command 包一层更短 deadline；`FastTimeoutPolicy()` 是面向低延迟生产路径的建议值。 |
| `Retry` | request-level retry 策略。默认 `RetryStrategyFailoverFirst`，失败后先换 host；`RetryStrategySameHostFirst` 只建议用于明确知道错误是瞬时抖动的私有节点。 |
| `Pool` | per-host idle 连接池。成功连接会复用，失败连接会关闭。 |
| `CircuitBreaker` | operation-aware 熔断。同一 host 某个 operation 失败进入冷却，不影响该 host 的其他 operation。 |
| `Observer` | 每次 request attempt 的事件 hook，可接日志、metrics、OpenTelemetry bridge。 |

### Timeout And Retry Policy

公网 TDX 节点的慢耗时通常不是“正常响应慢”，而是 connect/read timeout。2026-06-10 的 operation-host matrix 中，成功请求最大值约 `314ms`，而坏 host connect timeout 约 `1000ms`，BJ `security_list` read timeout 约 `6000ms`。生产路径建议：

| 场景 | 建议值 |
|---|---:|
| `Transport.ConnectTimeout` / `WriteTimeout` | `500-800ms` |
| quote/count/K 线/分时/财务/XDXR | `1-1.5s` |
| SH/SZ `security_list` 单页 | `1.5-2.5s` |
| report/block/company/file chunk | `2-3s` |
| transaction/history 类 | `2-3s` |
| BJ `security_list` | `800ms-1.5s` 后走 fallback |

`FastTimeoutPolicy()` 已内置这些分档中的保守低延迟版本，并对 `security_list + BJ` 做了更短的 market-specific timeout。

默认 retry 是 failover-first：每次失败后先尝试下一个 host。单元测试覆盖了一个坏 host、一个好 host 的场景，failover-first 在 `MaxAttempts=2` 下 `6/6` 成功；same-host-first 因把两次机会都耗在坏 host 上，成功率更低。same-host-first 适合自建私有节点或明确是瞬时半包/短抖动的场景，公网行情默认不建议开启。

组合接口会在内部每个 batch/page/chunk 之间检查父 context。业务侧应给全量列表、资金流、板块和文件下载这类重接口设置外层 `context.WithTimeout`，超时后不会继续调度新的内部请求。

## Markets And Categories

市场：

```go
model.MarketSH // 上海
model.MarketSZ // 深圳
model.MarketBJ // 北京
```

K 线周期：

```go
model.KlineMinute1
model.KlineMinute3
model.KlineMinute5
model.KlineMinute15
model.KlineMinute30
model.KlineMinute60
model.KlineDay
model.KlineWeek
model.KlineMonth
model.KlineSeason
model.KlineYear
```

证券标识：

```go
symbols := []model.Symbol{
    {Market: model.MarketSH, Code: "600519"},
    {Market: model.MarketSZ, Code: "000001"},
}
```

## API Usage

### Health And Server Selection

```go
servers := tdx.KnownServers()
results := tdx.PingAll(ctx, servers, tdx.TransportOptions{ConnectTimeout: 3 * time.Second})

client, err := tdx.FromBestHost(ctx, tdx.Options{Servers: servers})
if err != nil {
    // 没有 host 能完成 TCP/setup。
}
defer client.Close()

opClient, hostHealth, err := tdx.FromBestHostByOperations(ctx, tdx.Options{Servers: servers},
    command.NewSecurityListCommand(model.MarketSH, 0),
    command.NewSecurityQuotesCommand([]model.Symbol{{Market: model.MarketSH, Code: "600519"}}),
)
if err == nil {
    defer opClient.Close()
}

err = client.Ping(ctx)
stats := client.ServerStats()
opStats := client.OperationStats("security_list")
```

`PingAll` / `FromBestHost` 只证明 TCP/setup 可用。某个 host 能 ping 通，不代表它能稳定返回所有 operation。生产侧如果知道关键路径是证券列表、quote 或 K 线，可以用 `FromBestHostByOperations` 按 operation 探测并选择 host；返回的 `[]tdx.HostHealth` 会保留每个 host 的检查结果、latency 和错误。运行期继续看 `OperationStats`、`Observer` 和 metrics。

### Securities

```go
count, err := client.GetSecurityCount(ctx, model.MarketSH)

page, err := client.GetSecurityList(ctx, model.MarketSH, 0)

all, err := client.ListSecurities(ctx, model.MarketSH, model.MarketSZ, model.MarketBJ)
if tdx.IsPartialResultError(err) {
    // all.Failures 会保留 partial failure 信息。
} else if err != nil {
    // context canceled/deadline 或其他硬错误。
}

sample, err := client.ListSecuritiesWithOptions(ctx, tdx.ListSecuritiesOptions{
    Markets:           []model.Market{model.MarketSH, model.MarketSZ},
    MaxPagesPerMarket: 2,
    StopOnError:       false,
})

ashares, err := client.ListAShares(ctx)

asharesWithBJ, err := client.ListASharesWithOptions(ctx, tdx.ListSecuritiesOptions{
    Markets: []model.Market{model.MarketSH, model.MarketSZ, model.MarketBJ},
})

markets := client.ListMarkets(ctx)
```

返回类型：

- `[]model.Security`
- `model.PartialResult[model.Security]`

`ListSecurities` 会分页拉取，遇到部分市场失败时返回 typed partial result，不会静默丢失败信息。
`ListAShares` 默认只拉 SH/SZ，避免公网 BJ `security_list` 不稳定影响 instrument 主路径；需要 BJ 时通过 `ListASharesWithOptions` 显式传入 `model.MarketBJ`。
`ListSecuritiesWithOptions` / `ListASharesWithOptions` 可给全市场列表增加页数预算；达到 `MaxPagesPerMarket` 会返回 partial failure，而不是静默截断。
业务侧可用 `tdx.IsPartialResultError(err)` 区分“结果可读但不完整”和真正的硬失败，然后根据 `result.Failures` 决定是否重试某个 market/page。

### K Lines

```go
bars, err := client.GetSecurityBars(ctx, model.MarketSH, "600519", model.KlineDay, 0, 800)

indexBars, err := client.GetIndexBars(ctx, model.MarketSH, "000001", model.KlineDay, 0, 800)

autoBars, err := client.GetBars(ctx, model.MarketSH, "000001", model.KlineDay, 0, 800)
```

返回类型：`[]model.Bar`

`GetBars` 会根据 code 做股票/指数路由。K 线价格使用 `model.Decimal` 保存，避免 float 精度污染：

```go
fmt.Println(bars[0].Close.String())
fmt.Println(bars[0].Close.Float64())
```

### Quotes And Snapshot

```go
quotes, err := client.GetSecurityQuotes(ctx, []model.Symbol{
    {Market: model.MarketSH, Code: "600519"},
    {Market: model.MarketSZ, Code: "000001"},
})

snapshot, err := client.GetSnapshot(ctx, symbols)
```

返回类型：`[]model.Quote`

行为：

- 自动按协议上限分片，当前每批最多 `command.MaxQuoteBatch` 个 symbol。
- 保留五档买卖盘、成交量、成交额、涨速、server time、未知字段和 raw record。

常用字段：

```go
q := quotes[0]
fmt.Println(q.Code, q.Price.String(), q.Open.String(), q.High.String(), q.Low.String())
fmt.Println(q.Bid[0].Price.String(), q.Bid[0].Volume)
fmt.Println(q.Ask[0].Price.String(), q.Ask[0].Volume)
```

### Minute Time And Transactions

```go
minutes, err := client.GetMinuteTimeData(ctx, model.MarketSH, "600519")

historyMinutes, err := client.GetHistoryMinuteTimeData(ctx, model.MarketSH, "600519", 20260609)

txs, err := client.GetTransactionData(ctx, model.MarketSH, "600519", 0, 800)

historyTxs, err := client.GetHistoryTransactionData(ctx, model.MarketSH, "600519", 20260609, 0, 800)
```

返回类型：

- `[]model.MinuteTime`
- `[]model.Transaction`

逐笔成交会保留 `NumOrders`、`BuyOrSell`、`UnknownLast` 和 raw record，方便继续反推字段。

### Market Stat And Fund Flow

```go
stat, err := client.GetMarketStat(ctx)

flow, err := client.GetFundFlow(ctx, model.MarketSH, "600519")

fastFlow, err := client.GetFundFlowWithOptions(ctx, model.MarketSH, "600519", tdx.FundFlowOptions{
    PageSize:  800,
    MaxPages: 2,
})

historyFlow, err := client.GetHistoryFundFlow(ctx, model.MarketSH, "600519", 0, 20)
```

返回类型：

- `model.MarketStat`
- `model.FundFlow`
- `[]model.HistoricalFundFlow`

注意：

- `GetMarketStat` 是 canonical helper，基于 SH `880005` quote 派生。
- 当日 `GetFundFlow` 基于 L1 逐笔成交按金额阈值聚合。
- `GetHistoryFundFlow` 优先走 category 22 直连协议；若服务器空回包，则 fallback 到日 K 日期加历史逐笔成交重算。
- `GetFundFlowWithOptions` / `GetHistoryFundFlowWithOptions` 可设置 `PageSize`、`MaxStart`、`MaxPages`。达到页数预算会返回包含 `page budget` 的错误，避免长时间等待或静默截断。

```go
if tdx.IsPageBudgetError(err) {
    // 可换 host、降低业务级别，或把完整聚合放到后台任务。
}

fmt.Println(flow.MainNetInflow())
fmt.Println(flow.TotalNetInflow())
```

### Finance, XDXR, Company Info

```go
finance, err := client.GetFinanceInfo(ctx, model.MarketSH, "600519")

xdxr, err := client.GetXdxrInfo(ctx, model.MarketSH, "600519")

categories, err := client.GetCompanyInfoCategory(ctx, model.MarketSH, "600519")

content, err := client.GetCompanyInfoContent(ctx, model.MarketSH, "600519", "600519.txt", 0, 4096)
```

返回类型：

- `model.FinanceInfo`
- `[]model.XdxrRecord`
- `[]model.CompanyInfoCategory`
- `[]byte`

财务和 XDXR 解析借鉴并修复 pytdx/xmtdx 中已知问题：XDXR 记录头从当前 offset 读取，股本/股份类字段使用 TDX custom volume codec。

### Boards And Report Files

```go
boards, err := client.ListBoards(ctx, "concept") // concept/style/industry/index

members, err := client.ListBoardMembers(ctx, "某板块名称")

blockFileBoards, err := client.GetBlockInfo(ctx, "block_gn.dat")

baseInfoZip, err := client.GetReportFile(ctx, "base_info.zip")

sampleZip, err := client.GetReportFileWithOptions(ctx, "base_info.zip", tdx.FileFetchOptions{
    MaxChunks: 2,
})
```

返回类型：

- `[]model.Board`
- `[]string`
- `[]byte`

`GetBlockInfo` 会先读 metadata，再按 chunk 拉取板块文件并解析 `.dat`。`GetReportFile` 会按 chunk 拉取服务端文件，直到短 chunk。

`GetBlockInfoWithOptions`、`GetReportFileWithOptions`、`ListBoardsWithOptions`、`ListBoardMembersWithOptions` 可设置 `ChunkSize` 和 `MaxChunks`。达到 chunk 预算会返回包含 `chunk budget` 的错误，调用方可以据此选择缩短等待、换 host 或发起后台补全任务。`ChunkSize` 默认并限制为不超过 `tdx.DefaultFileChunkSize`。

```go
if tdx.IsChunkBudgetError(err) {
    // 当前调用触发了显式预算，不代表协议或 host 一定不可用。
}
```

需要统一处理所有预算类错误时可用 `tdx.IsBudgetError(err)`。

### Raw Capture For Protocol Auditing

```go
capture, err := client.Capture(ctx, command.NewSecurityQuotesCommand([]model.Symbol{
    {Market: model.MarketSH, Code: "600519"},
}))
```

`CapturedResponse` 会保留：

- operation
- server
- attempt
- latency
- request bytes
- 16-byte response header
- raw compressed body
- decoded body
- parsed result

可以写成 fixture：

```go
summary, err := diagnostic.WriteFixture("./fixtures/quote.fixture.json", capture)
```

## Observability

### Observer Hook

```go
client := tdx.NewClient(tdx.Options{
    Observer: tdx.ObserverFunc(func(event tdx.RequestEvent) {
        fmt.Printf(
            "op=%s host=%s attempt=%d ok=%v rows=%d latency=%s err=%s\n",
            event.Operation,
            event.Server.Addr(),
            event.Attempt,
            event.OK,
            event.Rows,
            event.Latency,
            event.Error,
        )
    }),
})
```

`RequestEvent` 字段：

| Field | 含义 |
|---|---|
| `Operation` | command operation 名称，例如 `security_list`。 |
| `Server` | 本次 attempt 使用的 host。 |
| `Attempt` | 当前请求第几次尝试。 |
| `OK` | attempt 是否成功。 |
| `Error` | 失败错误文本。 |
| `Latency` | attempt 耗时。 |
| `Rows` | parsed result 行数，slice/array 才会计数。 |
| `BodySize` | capture 路径下 decoded body 字节数。 |
| `Reused` | 是否复用了 idle pool 连接。 |

### Metrics Collector

```go
metrics := tdx.NewMetricsCollector()
client := tdx.NewClient(tdx.Options{Observer: metrics})

_ = client.Ping(ctx)
snapshots := metrics.Snapshot()
for _, s := range snapshots {
    fmt.Println(s.Operation, s.Server.Addr(), s.Attempts, s.Successes, s.Failures, s.TotalRows)
}
```

聚合维度是 `operation + host`，可用于上层导出 Prometheus/OpenTelemetry/Grafana。

## Diagnostics CLI

### Host Health

```bash
go run ./cmd/tdx-health
go run ./cmd/tdx-health -hosts 180.153.18.170:7709,180.153.18.171:7709 -timeout 5s
go run ./cmd/tdx-health -probe security-list-sh,quote -timeout 3s
```

不带 `-probe` 时输出 setup ping 结果；带 `-probe` 时按 operation 探测每个 host，输出 `selected` 和 `health` 明细。可用的 operation 名称与 `tdx-fixture-matrix` 默认矩阵一致，例如 `security-count`、`security-list-sh`、`security-list-sz`、`security-list-bj`、`stock-bars`、`index-bars`、`quote`、`minute`、`transaction`、`finance`、`xdxr`、`block-meta`、`report`。

### Operation Probe

```bash
go run ./cmd/tdx-probe -op security-count -market sh
go run ./cmd/tdx-probe -op quote -symbols sh:600519,sz:000001 -capture-dir ./fixtures/live
go run ./cmd/tdx-probe -op minute -market sh -code 600519 -capture-dir ./fixtures/live
go run ./cmd/tdx-probe -op history-transaction -market sh -code 600519 -date 20240607 -count 50 -capture-dir ./fixtures/live
go run ./cmd/tdx-probe -op report -file base_info.zip -count 30000 -capture-dir ./fixtures/live
```

支持的 `-op`：

```text
security-count
security-list
stock-bars
index-bars
quote
market-stat
minute
history-minute
transaction
history-transaction
fund-flow
history-fund-flow
finance
xdxr
company
block-meta
block
report
```

常用参数：

| Flag | 用途 |
|---|---|
| `-market sh|sz|bj` | 单市场 command 的市场。 |
| `-code 600519` | K 线、分时、逐笔、财务、除权、公司信息等 symbol command 的证券代码。 |
| `-symbols sh:600519,sz:000001` | quote 多 symbol/multi-market 探测，会按原始返回保存 fixture。 |
| `-date 20240607` | 历史分时、历史逐笔等历史 command 的交易日。 |
| `-start 0` / `-count 50` | 分页或 chunk command 的起点和数量。 |
| `-file base_info.zip` | 板块、报表文件 command 的文件名。 |

### Official Data Package Probe

`tdx-data-probe` 用于探测通达信官方 HTTP 数据包清单，当前主要服务于 BJ/全市场 fallback 调查。它不走 HQ `7709` TCP 协议，因此不要把它当作 `Client` 主链路的一部分；它是诊断和后续 fallback parser 的证据入口。

```bash
go run ./cmd/tdx-data-probe -timeout 15s -limit 8
go run ./cmd/tdx-data-probe -timeout 15s -prefix gpbj -limit 8
go run ./cmd/tdx-data-probe -kind local-index -timeout 15s -prefix gpbj -limit 8
curl -L --max-time 15 -sS https://data.tdx.com.cn/tdxgp/gpbj920021.dat -o /tmp/tdx-gpbj920021.dat
go run ./cmd/tdx-data-probe -kind dat13 -input /tmp/tdx-gpbj920021.dat -limit 8
go run ./cmd/tdx-data-probe -kind dat13 -input /tmp/tdx-gpbj920021.dat -limit 0
```

输出 JSON 摘要：

| Flag | 用途 |
|---|---|
| `-kind manifest|local-index|dat13` | `manifest` 解析 `filename,md5,size` 清单；`local-index` 解析 `.local` 文件中的 `[MD5]` 段；`dat13` 解析 13-byte raw record 样本。 |
| `-url` | 自定义官方数据包 URL；`local-index` 默认会切到 `https://data.tdx.com.cn/tdxgp/gpszsh.local`。 |
| `-input` | 从本地文件解析，适合用 curl 下载 `.dat` 后再让 Go parser 反推字段。 |
| `-prefix gpbj` | 按文件名前缀过滤，例如只看 BJ 候选 `gpbj*.dat`。 |
| `-limit 20` | JSON 中最多输出多少条 entry；`-1` 输出全部。 |

2026-06-10 live 观察：`gpszsh.txt` manifest 当前有 `7240` 个条目，其中 `gpbj*.dat` 为 `319` 个；`gpszsh.local` 的 `[MD5]` 段同样能枚举 `319` 个 `gpbj` 条目。manifest/local-index/HTTP 文件的 MD5 与 size 语义存在不一致样本，现阶段只作为诊断 finding，不作为强完整性校验。Go 标准 HTTP 客户端抓部分 `.dat` 时会收到 CDN `text/html` challenge，`tdx-data-probe` 会拒绝误解析；需要 `.dat` 样本时优先用 curl 下载，再用 `-input` 解析本地文件。`dat13` summary 会输出 marker 分布、按 marker 排序的 group 摘要、date-like 范围、float32-like 范围和非零 field2 计数，下一步应按 marker 分组反推字段。

### Live Fixture Matrix

```bash
TDX_LIVE=1 go run ./cmd/tdx-fixture-matrix \
  -out ./fixtures/live \
  -ops security-count,quote,history-fund-flow
```

输出 JSONL，一行一个 operation 结果。单个 operation 失败不会阻断后续 operation，适合收集公网节点能力矩阵。

### Live Integrity Validation

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

`tdx-validate` 会直接调用 public API，并输出 JSON 完整性报告：每个 operation 的 `ok`、行数、latency、error/warning finding 都会保留。默认需要 `TDX_LIVE=1`，避免普通测试误打公网。

### Operation Host Matrix

`tdx-op-matrix` 用于回答“某个失败是不是同一个公网节点导致的”。它会按 `host * operation * repeats` 逐项运行，每个 host 使用独立 client 且 `MaxAttempts=1`，因此不会被 failover 混淆。

```bash
TDX_LIVE=1 go run ./cmd/tdx-op-matrix \
  -hosts 180.153.18.170:7709,180.153.18.171:7709,115.238.56.198:7709 \
  -ops security-count,quote,security-list-bj,history-fund-flow,report \
  -repeats 2 \
  -operation-timeout 6s \
  -connect-timeout 1s
```

`tdx-op-matrix` 常用参数：

| Flag | 用途 |
|---|---|
| `-hosts` | 指定 host 列表；为空时使用 `KnownServers()`。 |
| `-ops` | 逗号分隔的 operation 名称，复用 `tdx-fixture-matrix` 的 operation 名称。 |
| `-repeats 2` | 每个 host/operation 跑几轮，用于小型压测和稳定性抽样。 |
| `-operation-timeout 6s` | 每个 host/operation 的独立超时。 |
| `-connect-timeout 1s` | TCP connect/write timeout。 |
| `-jsonl` | 输出每个 result 一行 JSONL，最后追加 summary 和 `timeout_recommendations`。 |

JSON 报告会包含 `timeout_recommendations`，按每个 host/operation 给出启发式推荐：成功样本使用 `max_latency * 4` 并做上下限夹取；没有成功样本的 timeout 型失败使用 fail-fast 推荐值。这个结果用于调参和观察，不会自动改客户端配置。

如果外层 `-timeout` 到期，`tdx-op-matrix` 会停止继续调度，输出已完成的 partial report，并在 JSON/JSONL summary 中标记 `canceled/error`。

2026-06-10 首轮 3 host、5 operation、2 repeats 压测结果：`180.153.18.171:7709` 对所有测试 operation 均 connect timeout；`security-list-bj` 在 `180.153.18.170:7709` 和 `115.238.56.198:7709` 上均 read timeout；`quote` 和 `security-count` 在这两个 host 上成功。因此失败不是同一节点单点问题，而是同时存在坏节点和 BJ list 的 operation/market 级失败。完整表见 [operation-host-matrix-2026-06-10.md](/Users/liuhanqing01/projects/tdx/docs/validation/operation-host-matrix-2026-06-10.md)。

常用参数：

| Flag | 用途 |
|---|---|
| `-markets sh,sz,bj` | 验证哪些市场的 count/list。 |
| `-symbols sh:600519,sz:000001` | 验证 quote/K 线/分时/逐笔/财务等使用的样本证券。 |
| `-kline day,week,month` | 验证哪些 K 线周期。 |
| `-full-kline` | 验证所有已知 K 线周期。 |
| `-full-security-list` | 逐页拉取所选市场的完整证券列表，输出 `security_list_<market>_page_<start>` 分页结果和 `security_list_<market>_full` 汇总结果；建议配合更长 `-operation-timeout`。 |
| `-security-list-page-retries 1` | full security-list 某页失败后额外重试该页的次数；每次 page 请求仍会使用 client 的 host failover。 |
| `-operation-timeout 8s` | 每个 operation 独立 timeout，避免一个慢接口拖垮整份报告。 |
| `-connect-timeout 1s` | TCP connect/write timeout，建议小于 operation timeout 以测试 failover。 |
| `-skip-boards` / `-skip-files` | 跳过板块和 report file 下载，适合先跑核心行情 smoke。 |

### Dump Raw Frame

```bash
go run ./cmd/tdx-dump-frame -hex <header-plus-body-hex>
```

### Compare With Python Reference

```bash
go run ./cmd/tdx-compare-py \
  -go ./fixtures/live/<go.fixture.json> \
  -py ./fixtures/pytdx/<py.json> \
  -max-diffs 100 \
  -tolerance 0.0001
```

`tdx-compare-py` 可以直接读取 Go fixture 的 `parsed_json` 字段，也可以比较普通 JSON 文件。

## Fault Tests

`tdxtest.StartScript` 可以模拟协议和网络故障，不依赖公网服务器：

```go
server, _ := tdxtest.StartScript(tdxtest.Script{
    Connections: []tdxtest.ConnectionScript{{
        Actions: []tdxtest.Action{
            tdxtest.ReadAndRespond(nil), // setup 1
            tdxtest.ReadAndRespond(nil), // setup 2
            tdxtest.ReadAndRespond(nil), // setup 3
            tdxtest.ReadAndBadZlib([]byte{1, 2, 3}, 8),
        },
    }},
})
defer server.Close()
```

可用动作：

| Action | 用途 |
|---|---|
| `ReadAndRespond(body)` | 读一个请求并返回正常 TDX frame。 |
| `ReadAndRaw(raw)` | 读一个请求并返回原始字节。 |
| `ReadAndBadZlib(raw, unzipSize)` | 返回 header 表示压缩，但 body 不是有效 zlib。 |
| `ReadAndPartialFrame(partialBody, declaredZipSize)` | 返回半包后断开。 |
| `ReadAndDelay(delay)` | 延迟。 |
| `ReadAndClose()` | 读请求后断开。 |

## Testing

```bash
go vet ./...
go test -count=1 ./...
```

Benchmarks：

```bash
go test -run=^$ -bench=. -benchmem ./codec ./frame ./command ./validation .
```

当前 benchmark 覆盖 codec、frame decode、核心 command parser、validation 规则，以及 `GetSecurityQuotes` 批量分片。性能基线应随 parser/高可用策略修改一起重新跑。

Live smoke：

```bash
go run ./cmd/tdx-probe -op security-count -market sh -timeout 5s
go run ./cmd/tdx-data-probe -prefix gpbj -limit 5
TDX_LIVE=1 go run ./cmd/tdx-validate -markets sh -symbols sh:600519 -kline day -skip-boards -skip-files
TDX_LIVE=1 go run ./cmd/tdx-validate -markets sh,sz -symbols sh:600519 -full-security-list -security-list-page-retries 1 -operation-timeout 30s -skip-boards -skip-files
```

Live fixture tests 不放进默认单元测试，请显式使用 `TDX_LIVE=1`。

2026-06-09 最新验证快照：

- `go test -count=1 ./...` 和 `go vet ./...` 均通过。
- `TDX_LIVE=1 tdx-validate -markets sh -symbols sh:600519 -skip-boards -skip-files`：12 项检查，10 OK，2 个公网超时错误，0 warnings；核心 count/list/quote/day-bar/minute/transaction/finance/xdxr/company 均通过。
- `TDX_LIVE=1 tdx-validate -markets sh,sz -symbols sh:600519,sz:000001 -skip-boards -skip-files`：14 项检查，12 OK，2 个公网超时错误，0 warnings；multi-market quote 返回 2 行并通过 symbol 完整性校验。
- `tdx-validate -full-security-list` 已支持全市场分页完整性校验；默认 smoke 为了速度仍只查第 0 页。最新 SH/SZ live baseline 加上 `-security-list-page-retries 1` 后分别拉完 27215/23411 行，若干分页首次失败后重试成功并保留 warning。
- BJ live baseline 中 `security_count_BJ=345`，但 `security_list_BJ_page_0` 在 15s timeout、3 次 page retry 下仍超时。`tdx-data-probe` 已确认官方 `tdxgp/gpszsh.txt` 与 `gpszsh.local` 可枚举 `319` 个 `gpbj*.dat` 候选，但还不能替代完整 BJ 证券列表。
- `tdx-op-matrix` 首轮压测显示 `180.153.18.171:7709` 是当前网络下的 host-level failure；但 `security-list-bj` 在可用 host 上也 read timeout，说明 BJ list 不是单一坏节点问题。
- `tdx-op-matrix` fast-timeout 压测使用 `-operation-timeout 2s -connect-timeout 700ms -repeats 3`，同样 3 host、5 operation，共 45 次 host/operation run，耗时 `24629ms`。成功请求最大 `215ms`；坏 host 约 `701ms` 失败；BJ list 在可用 host 上约 `2001ms` 失败。报告已输出 `timeout_recommendations`。
- `TDX_LIVE=1 tdx-validate` 含 boards/files：`boards_concept` 返回 270 行，`report_file_base_info.zip` 在当前公网节点返回 0 字节，仍需 fallback/节点矩阵继续反推。
- 性能基线在 Apple M2 / darwin arm64 上已重跑：quote parser 约 `34.0 us/op`，minute parser 约 `9.1 us/op`，5000 行 universe validation 约 `244.0 us/op`，80 符号 quote 分片 client benchmark 约 `15.5 us/op`。新增官方数据包 parser benchmark 中，7240 行 manifest 约 `1.72 ms/op`，7240 行 `.local` index 约 `1.31 ms/op`，10858 条 fixed13 raw record 约 `0.31 ms/op`。完整输出记录在 handoff 文档。

## Known Limits

- 通达信 HQ 协议不是官方公开协议，字段含义需要通过 fixture 持续反推。
- 公网 TDX 服务器按 operation/market 表现不一致，能握手不代表所有接口可用。
- BJ 全量列表在公网节点上不稳定，目前通过 partial result 和 report/base info fallback 预留处理空间。
- 官方 HTTP 数据包已能枚举 `gpbj*.dat` 并可用 `dat13` raw parser 观察 13-byte record，但字段语义、名称来源、MD5/size 语义仍需 fixture 和 parser 继续反推。
- 更丰富的扩展市场接口仍在待解析状态。
- 资金流中，当日资金流是逐笔成交聚合结果；历史资金流优先 category 22，空回包时 fallback 重算。

## Agent Checklist

给其他 RD 或 agent 接入时，按这个顺序走：

1. 先运行 `go run ./cmd/tdx-health` 看当前网络下可用 host。
2. 用 `tdx.NewClient(tdx.Options{})` 快速接入。
3. 生产任务设置 `Timeout`、`MaxAttempts`、`Pool`、`CircuitBreaker`、`Observer`。
4. 证券列表用 `ListSecurities` 或 `ListAShares`，读取 partial failures。
5. 批量快照用 `GetSecurityQuotes`，它会自动分片。
6. 需要反推协议字段时用 `Client.Capture` 或 `tdx-probe -capture-dir`。
7. 调查官方数据包 fallback 时用 `tdx-data-probe -prefix gpbj` 和 `tdx-data-probe -kind local-index -prefix gpbj`。
8. 判断公网节点/operation 是否稳定时用 `tdx-op-matrix`，重点看 per-host success rate 和 last error。
9. 和 pytdx/xmtdx 对照时用 `tdx-compare-py`。
10. 遇到故障复现时用 `tdxtest.StartScript` 写 fake server 测试。
11. 发布前用 `tdx-validate` 跑 live 完整性报告，并用 `go test -bench=. -benchmem` 更新性能基线。
