# Broader TDX Ecosystem Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add broader TDX ecosystem capability through isolated, evidence-backed protocol tracks without destabilizing the existing HQ `7709` client.

**Architecture:** Keep `github.com/quantbeing/tdx` root focused on HQ `7709`. Add new packages for ExHQ, VIPDOC, and professional financial packages; keep ranking/Level2/trading as gated research until fixtures and entitlements exist.

**Tech Stack:** Go, existing `codec`/`frame` patterns, pytdx/mootdx as reference implementations, fixture JSON, fake TCP server tests, local temp-file fixtures.

---

## Current Status

- [x] Task 1-3 ExHQ experimental package implemented with offline parser/client tests.
- [x] Task 4 VIPDOC package implemented for `.day` and block `.dat`; minute parsing is explicit unsupported pending format verification.
- [x] Task 5 Professional financial package parser/downloader implemented with size limits and hostile fixture tests.
- [x] Task 6 Raw discovery mode implemented as `tdx-probe -raw-hex`.
- [x] Task 7 offline validation reports added for ExHQ, VIPDOC, and financepkg.
- [ ] ExHQ live validation remains pending and must be opt-in.

## File Structure

- Create `docs/investigations/tdx-broader-ecosystem-research-2026-06-11.md`: research source matrix and track decisions.
- Create `docs/superpowers/specs/2026-06-11-broader-tdx-ecosystem-design.md`: design boundaries for each track.
- Create `exhq/`: future ExHQ protocol package.
- Create `exhq/command/`: ExHQ command builders/parsers.
- Create `exhq/model/`: ExHQ-specific market, instrument, quote, bar, minute, and transaction models.
- Create `vipdoc/`: local TDX file parser package.
- Create `financepkg/`: professional financial package parser/downloader helpers.
- Modify `diagnostic/`: add raw command capture only when implementing ranking/sorting discovery.
- Keep `client.go`, root `command/`, and root `model/` stable unless a shared helper is truly generic.

## Task 1: ExHQ Model And Command Fixtures

**Files:**

- Create: `exhq/model/types.go`
- Create: `exhq/command/interface.go`
- Create: `exhq/command/markets.go`
- Create: `exhq/command/markets_test.go`

- [ ] **Step 1: Write market parser test**

```go
func TestParseMarkets(t *testing.T) {
	body := make([]byte, 2+64)
	binary.LittleEndian.PutUint16(body[0:2], 1)
	body[2] = 1
	copy(body[3:35], []byte("Futures\x00"))
	body[35] = 47
	copy(body[36:38], []byte("IF"))

	got, err := command.ParseMarkets(body)
	if err != nil {
		t.Fatalf("ParseMarkets: %v", err)
	}
	if len(got) != 1 || got[0].MarketID != 47 || got[0].Category != 1 || got[0].Name != "Futures" {
		t.Fatalf("markets = %+v", got)
	}
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./exhq/command -run TestParseMarkets -count=1`

Expected: FAIL because `exhq/command` does not exist.

- [ ] **Step 3: Implement minimal ExHQ model and market parser**

```go
package model

type Market struct {
    MarketID uint8
    Category uint8
    Name string
    ShortName string
    Unknown []byte
    Raw []byte
}
```

```go
package command

type Command interface {
    BuildRequest() ([]byte, error)
    ParseResponse([]byte) (any, error)
    Operation() string
}
```

`ParseMarkets` must read a `uint16` count, then 64-byte rows: `category:uint8`, `name[32]`, `market:uint8`, `shortName[2]`, 26 skipped bytes, 2 unknown bytes. Decode names with existing `codec.DecodeGBKBestEffort`.

- [ ] **Step 4: Verify**

Run: `go test ./exhq/command -run TestParseMarkets -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add exhq/model/types.go exhq/command/interface.go exhq/command/markets.go exhq/command/markets_test.go
git commit -m "feat: add exhq market parser"
```

## Task 2: ExHQ Transport And Client Skeleton

**Files:**

- Create: `exhq/client.go`
- Create: `exhq/transport.go`
- Create: `exhq/client_test.go`
- Modify: `tdxtest/fakeserver.go` only if reusable frame scripting needs a small extension.

- [ ] **Step 1: Write setup/fake-server client test**

Test should start a local script server that expects the ExHQ setup request and a markets request, then returns a normal TDX frame containing one market row. The client call `GetMarkets(ctx)` should return that row.

- [ ] **Step 2: Run failing test**

Run: `go test ./exhq -run TestClientGetMarkets -count=1`

Expected: FAIL because client is missing.

- [ ] **Step 3: Implement client skeleton**

Implement `Options`, `Client`, `NewClient`, `Close`, `GetMarkets`, and a single TCP round tripper. Reuse the existing `frame.ReadResponse` style if it is exported; otherwise copy only the small frame-reading logic needed and add a follow-up task to extract a shared internal helper.

- [ ] **Step 4: Verify**

Run: `go test ./exhq -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add exhq/client.go exhq/transport.go exhq/client_test.go tdxtest/fakeserver.go
git commit -m "feat: add exhq client skeleton"
```

## Task 3: ExHQ Core Commands

**Files:**

- Create: `exhq/command/instruments.go`
- Create: `exhq/command/quotes.go`
- Create: `exhq/command/bars.go`
- Create: `exhq/command/timeseries.go`
- Create tests beside each file.
- Modify: `exhq/client.go`

- [ ] **Step 1: Add parser tests from synthetic fixtures**

Add table-driven tests for:

- instrument count
- instrument info
- single quote
- quote list
- bars
- minute
- transaction

Each test must include at least one unknown field assertion and one raw-row assertion.

- [ ] **Step 2: Run failing tests**

Run: `go test ./exhq/command -count=1`

Expected: FAIL for missing command files.

- [ ] **Step 3: Implement command builders/parsers**

Use pytdx parser request bytes as references. Keep fields conservative: only name fields that pytdx clearly parses; expose the rest as `UnknownN` or raw byte slices.

- [ ] **Step 4: Add client methods**

Expose methods listed in `docs/investigations/tdx-broader-ecosystem-research-2026-06-11.md`.

- [ ] **Step 5: Verify**

Run: `go test ./exhq/... -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add exhq
git commit -m "feat: add exhq core commands"
```

## Task 4: VIPDOC Parser Package

**Files:**

- Create: `vipdoc/reader.go`
- Create: `vipdoc/daily.go`
- Create: `vipdoc/minute.go`
- Create: `vipdoc/block.go`
- Create: `vipdoc/errors.go`
- Create tests beside each parser file.

- [ ] **Step 1: Write local temp-file tests**

Use `t.TempDir()` to create:

- `vipdoc/sh/lday/sh600000.day` with one `<IIIIIfII` record.
- `vipdoc/sh/block/block_gn.dat` style bytes with offset 384 and one block.
- A truncated file to assert parse errors include path and offset.

- [ ] **Step 2: Run failing tests**

Run: `go test ./vipdoc -count=1`

Expected: FAIL because package is missing.

- [ ] **Step 3: Implement daily and block parsers**

Daily parser should decode date, open, high, low, close, amount, volume, and raw bytes. Block parser should mirror existing root board parser where possible, but remain local-file based.

- [ ] **Step 4: Implement minute parser**

Implement 1-minute and 5-minute parser only after confirming pytdx `min_bar_reader.py` and `lc_min_bar_reader.py` record layouts. If not confirmed, leave minute out of this commit and keep the test skipped with a source link and reason.

- [ ] **Step 5: Verify**

Run: `go test ./vipdoc -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add vipdoc
git commit -m "feat: add vipdoc local readers"
```

## Task 5: Professional Financial Package Parser

**Files:**

- Create: `financepkg/package.go`
- Create: `financepkg/package_test.go`
- Create: `financepkg/download.go`
- Create: `financepkg/download_test.go`

- [ ] **Step 1: Write parser fixture test**

Create an in-memory zip containing one `.dat` with:

- header `<1hI1H3L`
- one stock item `<6s1c1L`
- two float report fields

Assert code, report date, field count, values, and raw bytes.

- [ ] **Step 2: Run failing test**

Run: `go test ./financepkg -run TestParsePackage -count=1`

Expected: FAIL because package is missing.

- [ ] **Step 3: Implement parser**

Implement `ParsePackage(data []byte) (*Package, error)` with zip and raw dat support. Return typed errors for missing `.dat`, invalid header, short stock item, and short field payload.

- [ ] **Step 4: Add downloader adapter tests**

Define a tiny interface:

```go
type ReportFileClient interface {
    GetReportFile(ctx context.Context, filename string) ([]byte, error)
}
```

Test `ListPackages` parsing `tdxfin/gpcw.txt` content and `DownloadPackage` calling `tdxfin/<filename>`.

- [ ] **Step 5: Verify**

Run: `go test ./financepkg -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add financepkg
git commit -m "feat: add financial package parser"
```

## Task 6: Raw Discovery Tooling

**Files:**

- Modify: `cmd/tdx-probe/main.go`
- Modify: `cmd/tdx-probe/main_test.go`
- Modify: `README.md`
- Modify: `docs/api/protocol-interface-map.md`

- [ ] **Step 1: Write CLI test for raw hex**

Add test that `tdx-probe -raw-hex 0c... -capture-dir tmp` rejects invalid hex and does not require a known operation.

- [ ] **Step 2: Run failing CLI test**

Run: `go test ./cmd/tdx-probe -run RawHex -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement unsupported/raw command**

Add a diagnostic-only command type with `Operation() == "raw_probe"` that writes caller bytes and returns decoded body bytes without parsing.

- [ ] **Step 4: Document warning**

Document that raw command probing is for owned servers/legal captures only and is not a stable API.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./cmd/tdx-probe -count=1
go test ./diagnostic -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/tdx-probe README.md docs/api/protocol-interface-map.md
git commit -m "feat: add raw protocol probe mode"
```

## Task 7: Live Evidence And Docs

**Files:**

- Create: `docs/validation/exhq-live-matrix-YYYY-MM-DD.md`
- Create: `docs/validation/vipdoc-fixtures-YYYY-MM-DD.md`
- Create: `docs/validation/financepkg-fixtures-YYYY-MM-DD.md`
- Modify: `docs/api/capability-matrix.md`
- Modify: `README.md`

- [ ] **Step 1: Run offline tests**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 2: Run ExHQ live smoke only if enabled**

Run: `TDX_EXHQ_LIVE=1 go test ./exhq -run Live -count=1`

Expected: PASS when an ExHQ server is reachable, SKIP otherwise.

- [ ] **Step 3: Run parser benchmarks**

Run: `go test -run=^$ -bench=. -benchmem ./exhq/... ./vipdoc ./financepkg`

Expected: PASS.

- [ ] **Step 4: Update capability docs**

Mark ExHQ/VIPDOC/financepkg as experimental until live and fixture evidence meets acceptance gates.

- [ ] **Step 5: Commit**

```bash
git add docs README.md
git commit -m "docs: record broader ecosystem validation"
```
