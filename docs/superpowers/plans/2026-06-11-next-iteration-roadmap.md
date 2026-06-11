# TDX Go Library Next Iteration Roadmap

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the current HQ `7709` Go library from a usable v0 core into a better validated, easier to consume, and more complete production SDK.

**Architecture:** Keep the stable HQ TCP core intact. Iterate in layers: release hygiene first, then live evidence, parser comparison, fallback research, and finally broader TDX ecosystem surfaces.

**Tech Stack:** Go, TDX HQ TCP `7709`, existing diagnostic CLIs, fixture JSON, pytdx/xmtdx comparison scripts, fake TDX server tests.

---

## Iteration Directions

### 1. Release And Consumption Readiness

Current local `main` is ahead of `origin/main` with post-`v0.1.1` API and CLI improvements. Before larger protocol work, publish this state and cut a tag so other projects can depend on a stable version.

Deliverables:

- [ ] Push current commits to `origin/main`. Owner: user.
- [x] Run final release verification.
- [ ] Tag `v0.1.2` after push. Owner: user.
- [ ] Verify a fresh external module can `go get github.com/quantbeing/tdx@v0.1.2` after tag is pushed.
- [x] Update README release notes or changelog section.

Acceptance:

- `go test -count=1 ./...`
- `go vet ./...`
- `go test -race .`
- `go test -run=^$ -bench=Client -benchmem .`
- External consumer imports `tdx`, calls `ListAShares`, `GetSecurityBars`, `GetSecurityQuotes`, and compiles.

### 2. Live Evidence Refresh

The latest live evidence is useful but now slightly behind the retry/timeout/CLI changes. Refresh the operation matrix and validation reports using the new flags.

Deliverables:

- [ ] Run `tdx-op-matrix` for core operations with JSONL output.
- [ ] Run `tdx-validate` for SH/SZ default ingestion.
- [ ] Run `tdx-fixture-matrix` for quote, minute, transaction, finance, xdxr, company, board/report surfaces.
- [ ] Save reports under `docs/validation/` with exact command lines.
- [ ] Capture fixtures for any parser warning or mismatch.

Acceptance:

- Current table of interface latency and success rates exists.
- Known unstable operations are classified by host-level failure vs operation/market failure.
- No README claim relies only on old pre-retry-control data.

### 3. Python Reference Comparison

The Go parser is useful, but stronger evidence comes from pytdx/xmtdx comparison on the same symbols and fixtures.

Deliverables:

- [ ] Reuse `/Users/liuhanqing01/projects/quantbeing/scripts/tdx` to capture pytdx/xmtdx JSON for the same operations.
- [ ] Run `tdx-compare-py` against Go fixtures.
- [ ] Classify differences into parser bugs, unknown fields, naming/format differences, and acceptable numeric tolerance.
- [ ] Add fixture-backed regression tests for real parser bugs.

Acceptance:

- Comparison report exists for quote, K-line, minute, transaction, finance, xdxr, and company category/content.
- Any parser fix includes binary fixture and unit test.

### 4. Block/Report File And History Fund-Flow Hardening

Current block/report and history fund-flow are partial mainly because public servers can return empty report payloads or transaction timeouts.

Deliverables:

- [ ] Run host/operation matrix specifically for `report`, `block-meta`, `block`, `transaction`, `history-transaction`, and `history-fund-flow`.
- [ ] Add richer diagnostics for empty file payloads: host, filename, chunk offset, header sizes, body sizes.
- [ ] Capture successful report/block fixtures from any working host.
- [ ] Add parser fixtures for board/report data when successful payloads exist.
- [ ] Revisit history fund-flow fallback budgets with live data.

Acceptance:

- Empty `base_info.zip` behavior is documented as host/file specific or global.
- History fund-flow timeout behavior has current host matrix evidence.

### 5. BJ Universe Fallback Research

This remains a long-running research stream. Do not block normal A-share SH/SZ ingestion on it.

Deliverables:

- [ ] Continue `tdx-data-probe` exploration of `gpbj*.dat` manifest and local-index candidates.
- [ ] Decode fixed 13-byte records by marker group.
- [ ] Compare candidate files with `security_count_BJ=345`.
- [ ] Investigate `base_info.zip` or related report/security files as protocol-accessible fallback.
- [ ] Design `ListASharesWithOptions(...BJ...)` merge semantics with typed partial failures.

Acceptance:

- A BJ fallback can produce code/name/status metadata with confidence, or a document explicitly proves why the available data package is insufficient.

### 6. API Surface Polish

The public API is already usable, but a few ergonomics improvements would make it friendlier for other RD/agents.

Deliverables:

- [x] Add examples under `examples/` for instrument ingestion, quote batch, K-line, diagnostics, and private-node retry policy.
- [x] Consider an `internal/cmdflags` helper if retry flag parsing keeps duplicating across CLI packages.
- [x] Add a changelog or release notes file.
- [x] Add package-level Go examples that run under `go test`.

Acceptance:

- [x] A new project can copy one example and run it with minimal interpretation.
- [x] CLI flag docs and examples are consistent with actual flags.

### 7. Broader TDX Ecosystem Research

This is outside the current HQ `7709` stable core and should be planned as separate protocol tracks.

Candidate tracks:

- Level2 depth, order queue, order detail.
- Ranking lists, market sorting,异动/竞价.
- Extension market, futures, options, HK/US surfaces.
- Trading protocol.
- Local VIPDOC file parsing.
- Professional finance package parsing.

Acceptance:

- Each track starts with protocol source research, fixture capture, command matrix, and clear scope boundaries before production API exposure.

## Recommended Execution Order

1. Release and consumption readiness.
2. Live evidence refresh.
3. Python reference comparison.
4. Block/report and history fund-flow hardening.
5. BJ universe fallback research.
6. API examples and release notes.
7. Broader TDX ecosystem tracks.

## Notes

- Keep `ListAShares()` default SH/SZ only until BJ fallback is proven stable.
- Keep `tdx-op-matrix` at `MaxAttempts=1` per host unless the tool explicitly adds a separate mode; its purpose is host/operation evidence, not best-effort success.
- Use request-level `WithRequestOptions` for one-off SLA changes, not new retry parameters on every data API.
- Commit live fixtures only when they are needed to explain or protect parser behavior.
