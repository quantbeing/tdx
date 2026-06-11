# ExHQ Live Matrix

Date: 2026-06-11

Status: not run against public ExHQ servers in this cycle.

## Implemented Offline Coverage

- Package: `github.com/quantbeing/tdx/exhq`
- Commands covered by offline tests:
  - markets
  - instrument count
  - instrument info
  - instrument quote
  - instrument quote list
  - bars
  - history bars range
  - minute
  - history minute
  - transaction
  - history transaction
- Transport tests cover setup, frame decode, write timeout, read timeout, context cancellation, and close wakeup behavior.

## Verification Run

```bash
go test ./exhq/... -count=1
```

Result: passed locally on 2026-06-11.

## Live Command To Run Next

```bash
TDX_EXHQ_LIVE=1 go test ./exhq -run Live -count=1
```

No live ExHQ test is currently enabled in the default suite. Before promoting ExHQ from experimental to stable, add an opt-in live smoke that checks at least:

- `GetMarkets`
- `GetInstrumentCount`
- `GetInstrumentInfo`
- one known quote
- one K-line request
- one minute/transaction request if the selected server supports it

## Release Boundary

ExHQ remains experimental until current public or private `7727` hosts are validated and at least one live fixture is captured for the core commands.
