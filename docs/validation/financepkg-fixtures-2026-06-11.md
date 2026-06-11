# Finance Package Fixture Validation

Date: 2026-06-11

Status: offline synthetic fixtures passed.

## Implemented Coverage

- Package: `github.com/quantbeing/tdx/financepkg`
- `ParsePackage` supports:
  - zip payload containing one `.dat` file
  - raw `.dat` payload
- Parsed format:
  - header `<1hI1H3L`
  - stock item `<6s1c1L`
  - float32 report fields
- Downloader helpers:
  - `ListPackages(ctx, client)` reads `tdxfin/gpcw.txt`.
  - `DownloadPackage(ctx, client, meta)` reads `tdxfin/<filename>`.

## Safety Coverage

- Typed parse errors for missing `.dat`, invalid header, short stock item, and short field payload.
- Size guard for `.dat` payloads.
- Size guard for total decoded field bytes.
- Rejection for field offsets inside the stock-item table.
- Rejection for reused field offsets that could amplify decoded allocations.
- Zip entry size check plus `io.LimitedReader` during extraction.

## Verification Run

```bash
go test ./financepkg -count=1
```

Result: passed locally on 2026-06-11.

## Release Boundary

The parser intentionally exposes financial values as `[]float32` without a built-in field dictionary. A field dictionary should be added only after a separate audited source is linked and tested.
