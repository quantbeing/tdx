# VIPDOC Fixture Validation

Date: 2026-06-11

Status: offline synthetic fixtures passed.

## Implemented Coverage

- Package: `github.com/quantbeing/tdx/vipdoc`
- `.day` parser:
  - Parses local `vipdoc/{sh|sz}/lday/{sh|sz}{code}.day`.
  - Uses 32-byte `<IIIIIfII` records.
  - Keeps raw price integers and full raw record bytes.
  - Applies conservative SH/SZ A-share price scale `0.01`.
- block `.dat` parser:
  - Starts at offset `384`.
  - Reads `uint16` block count.
  - Parses block name, stock count, block type, and 7-byte codes.
  - Keeps raw code and raw block bytes.
- Minute parser:
  - Returns explicit unsupported errors for 1/5 minute files until the local file record layout is verified.

## Safety Coverage

- Rejects invalid root paths.
- Rejects symbol codes with path separators or dot components.
- Rejects block filenames that are absolute or escape the configured root.
- Reports truncated files with path, offset, expected bytes, and actual bytes.
- Reports invalid block stock count with `stock_count` and `max` context.

## Verification Run

```bash
go test ./vipdoc -count=1
```

Result: passed locally on 2026-06-11.

## Release Boundary

VIPDOC daily and block readers are usable as experimental local parsers. Minute readers remain intentionally unsupported until real local fixtures and format confirmation are added.
