# TDX Python Protocol Implementation Notes

Date: 2026-06-09

## References

- `pytdx`: original Python protocol implementation; commands build binary request bytes and parse binary response bodies.
- `xmtdx`: modern zero-dependency Python rewrite with clean `transport` / `commands` / `codec` / `models` boundaries.
- `mootdx`: convenient wrapper around pytdx-style APIs; useful for API ergonomics, less useful as the lowest-level protocol source.

## Protocol Shape

- TDX HQ uses TCP, usually port `7709`.
- A connection must send three setup commands before normal requests.
- Each response has a fixed 16-byte header: three unknown uint32 fields, `zipsize`, and `unzipsize`.
- Body bytes are zlib-compressed when `zipsize != unzipsize`.
- Prices in K-line and quote payloads use a signed variable-length integer.
- Volumes, amounts, pre-close fields, and share-count fields use the TDX 4-byte custom float.
- Names and many text fields are GBK encoded.

## pytdx Issues To Avoid

- Do not drop unknown fields.
- Decode GBK with replacement behavior instead of crashing on truncated bytes.
- Do not decode `pre_close` as a simple integer price.
- Preserve transaction trailing fields.
- Preserve minute-time unknown fields.
- Decode xdxr share-count fields with the custom volume codec.
- Do not trust quote protocol slots for limit-up/limit-down prices without a rule-based calculation.

## Current Go v0 Status

- Implemented: frame header/body, GBK, price varint, volume codec, datetime codec, setup bytes, per-host idle connection pool, security count, security list, stock bars, index bars, quote/snapshot, minute time, transactions, market-stat derivation, today fund-flow aggregation, history fund-flow category 22 parser/fallback, finance info, xdxr info, company info, block/report file fetch, request-level failover, operation-aware circuit breaker, server stats, heartbeat manager, raw fixture capture, live fixture matrix capture, and JSON comparison diagnostics.
- Exposed but not fully decoded yet: richer extended-market commands.
- The unsupported surfaces intentionally return typed errors so the public API can stabilize while protocol fixtures are added.
