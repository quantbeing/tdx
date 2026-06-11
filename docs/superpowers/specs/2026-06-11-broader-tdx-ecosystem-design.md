# Broader TDX Ecosystem Design

Date: 2026-06-11

## Goal

Expand beyond the current HQ `7709` market-data client without weakening the stability or clarity of the existing package. Broader TDX capabilities must enter as separately researched protocol tracks, each with evidence, fixtures, tests, and a narrow API boundary.

## Scope

In scope:

- ExHQ `7727` extended market protocol research and prototype.
- Local VIPDOC file readers.
- Professional financial package download and parsing.
- Diagnostic-only raw command discovery for ranking/sorting/market-board surfaces.
- Level2 feasibility notes and entitlement gate.
- Trading protocol boundary decision.

Out of scope for the first implementation cycle:

- Stable Level2 API without an entitled endpoint.
- Trading APIs inside the market-data package.
- Public ranking/sorting APIs without binary fixtures.
- Any API that silently guesses fields without raw bytes and unknown fields preserved.

## Architecture

Keep the root `tdx` package focused on HQ `7709`. New capability families should live in separate packages:

- `exhq`: TCP transport and commands for extended markets.
- `vipdoc`: local file parsers with no network dependency.
- `financepkg`: parser and downloader helpers for professional financial packages.
- `diagnostic`: raw command capture extensions shared by discovery tracks.
- `level2`: reserved future package gated by entitlement and live fixtures.

Shared concepts such as decimal values, GBK decoding, raw byte retention, and fixture writing should be reused. Wire command structs should not be shared between HQ and ExHQ unless the request and response bytes are proven identical.

## Track Design

### ExHQ

ExHQ gets its own client because pytdx uses different setup commands and port `7727`. The first prototype should implement markets, instrument count, instrument info, single quote, quote list, bars, minute, history minute, transaction, and history transaction. It should use the same operational posture as HQ: per-operation stats, failover, capture, raw bytes, and typed unknown fields.

The public model should avoid reusing `model.MarketSH/SZ/BJ`; ExHQ market ids are independent numeric ids.

### VIPDOC

VIPDOC is a local reader package. It should accept a caller-provided root path and expose deterministic parsers for `.day`, minute files, extended-market files, block files, custom blocks, and corporate-action files. Errors must report filename and byte offset so broken local installations can be diagnosed quickly.

### Finance Packages

Professional financial package support should start with parser-only tests, then add download through the existing report-file command. The package list comes from `tdxfin/gpcw.txt`; package payloads are zip files containing `.dat` records. Unknown financial columns should remain `FieldN` until a separately audited dictionary exists.

### Ranking And Sorting Discovery

Ranking/sorting/auction data should start as raw probe tooling. A command should not become an API until request bytes, response framing, row count, and at least a minimal field map are fixture-backed.

### Level2

Level2 needs a hard entitlement gate. The library should not pretend L1 quote data is Level2, and it should not expose a stable API until a known Level2 server or credential path is available for live smoke tests.

### Trading

Trading is a different risk class. pytdx points to a `trade.dll` wrapper rather than a pure market-data TCP protocol. Keep it out of this module unless the project intentionally creates a separate trading module with authentication, audit logging, and risk controls.

## Testing Strategy

- Parser fixtures first for every binary format.
- Fake TCP server for ExHQ setup, frame decode, timeout, bad zlib, half-frame, and empty response cases.
- Local temp-directory fixtures for VIPDOC path resolution and corrupted file tests.
- Live tests must remain opt-in behind environment flags.
- Comparison scripts should be added only after a Python reference fixture exists.

## Release Strategy

Release broader tracks behind experimental package names and docs. Do not add them to the root README quickstart until the track has live evidence and fixture-backed parser coverage. Stable promotion requires API docs, examples, and validation reports.
