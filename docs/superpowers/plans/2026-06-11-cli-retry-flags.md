# CLI Retry Flags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let diagnostic and validation CLI users configure request attempts and retry strategy without changing Go code.

**Architecture:** Keep library defaults in `tdx.Options`, expose CLI flags that fill `Options.MaxAttempts` and `Options.Retry` for tools that use normal failover. Do not change `tdx-op-matrix` because it intentionally uses `MaxAttempts=1` per host to preserve host/operation evidence.

**Tech Stack:** Go `flag`, existing `tdx.Options`, command package tests, README CLI docs.

---

### Task 1: Shared Retry Flag Semantics

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-validate/main.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-probe/main.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-health/main.go`

- [ ] **Step 1: Add consistent flag names**

Each CLI should expose:

```text
-max-attempts int
-retry-strategy string
-same-host-attempts int
```

Defaults:

```text
max-attempts=0
retry-strategy=failover-first
same-host-attempts=1
```

- [ ] **Step 2: Parse aliases**

Accept these values:

```text
failover-first
failover_first
failover
same-host-first
same_host_first
same-host
```

Unknown values must return an error for `tdx-validate`, and JSON error plus exit code 2 for `tdx-probe` / `tdx-health`.

### Task 2: Wire Options

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-validate/main.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-probe/main.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-health/main.go`

- [ ] **Step 1: tdx-validate**

Update `buildClientOptions` to fill:

```go
Options{
	MaxAttempts: maxAttempts,
	Retry: tdx.RetryOptions{
		Strategy: retryStrategy,
		SameHostAttempts: sameHostAttempts,
	},
}
```

Keep existing operation/connect/read/write timeout behavior.

- [ ] **Step 2: tdx-probe**

Use the parsed retry options when constructing:

```go
tdx.NewClient(tdx.Options{...})
```

This applies to both `HealthCheck` and `Capture`.

- [ ] **Step 3: tdx-health**

Use the parsed retry options for `FromBestHostByOperations` probe mode. Leave ordinary `PingAll` unchanged because it tests raw setup reachability for each host.

### Task 3: Tests

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-validate/main_test.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-probe/main_test.go`
- Modify: `/Users/liuhanqing01/projects/tdx/cmd/tdx-health/main_test.go`

- [ ] **Step 1: Validate CLI option construction**

Add a test that calls `buildClientOptions` with `maxAttempts=3`, `same-host-first`, `sameHostAttempts=2`, then asserts:

```go
opts.MaxAttempts == 3
opts.Retry.Strategy == tdx.RetryStrategySameHostFirst
opts.Retry.SameHostAttempts == 2
```

- [ ] **Step 2: Retry strategy parser**

Add tests for aliases and unknown values in at least one CLI package. If helpers are duplicated, test each package's helper.

- [ ] **Step 3: Run focused tests**

Run:

```bash
go test ./cmd/tdx-validate ./cmd/tdx-probe ./cmd/tdx-health
```

### Task 4: README

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/README.md`

- [ ] **Step 1: Document flags**

Near the existing diagnostic CLI sections, mention:

```text
-max-attempts
-retry-strategy failover-first|same-host-first
-same-host-attempts
```

Explain that public nodes should usually use failover-first, while same-host-first is mainly for private nodes or transient short failures.

### Task 5: Verification And Commit

**Files:**
- Verify all modified files.

- [ ] **Step 1: Format**

Run:

```bash
gofmt -w cmd/tdx-validate/main.go cmd/tdx-validate/main_test.go cmd/tdx-probe/main.go cmd/tdx-probe/main_test.go cmd/tdx-health/main.go cmd/tdx-health/main_test.go
```

- [ ] **Step 2: Test**

Run:

```bash
go test ./cmd/tdx-validate ./cmd/tdx-probe ./cmd/tdx-health
go test -count=1 ./...
go vet ./...
```

- [ ] **Step 3: Commit**

Run:

```bash
git add cmd/tdx-validate cmd/tdx-probe cmd/tdx-health README.md docs/superpowers/plans/2026-06-11-cli-retry-flags.md
git commit -m "feat: add retry controls to diagnostic CLIs"
```

Expected: commit succeeds; `.idea/` remains untouched.

## Self-Review

- Spec coverage: covers validate/probe/health flags, parser aliases, tests, docs, and verification.
- Scope decision: `tdx-op-matrix` remains unchanged because `MaxAttempts=1` is part of its measurement design.
- Placeholder scan: no TODO/TBD/fill-in-later items.

## Addendum: Fixture Matrix

After the first CLI pass, `tdx-fixture-matrix` was added to the same retry-control family because it captures raw protocol fixtures through `Client.Capture` and benefits from the same public/private node diagnostics:

- `-max-attempts`
- `-retry-strategy failover-first|same-host-first`
- `-same-host-attempts`

`tdx-op-matrix` remains intentionally unchanged.
