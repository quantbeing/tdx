# Request Options Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow callers to override retry, attempt, and timeout policy for one request chain through `context.Context`, without adding retry parameters to every market-data API.

**Architecture:** Keep production defaults on `tdx.Options`. Add a tiny context-carried `RequestOptions` layer that `Client.execute` and `Client.Capture` read when they build attempt plans and command deadlines. Request timeout overrides are an overlay, so unspecified operations inherit the client policy. Composite API business budgets stay on the existing `XxxWithOptions` structs.

**Tech Stack:** Go, `context`, existing `Client` transport/failover path, existing fake dialer tests.

---

### Task 1: Public Request Option API

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/client.go`
- Test: `/Users/liuhanqing01/projects/tdx/client_test.go`

- [ ] **Step 1: Add the public type and context helpers**

Add this near the existing `Options`, `RetryOptions`, and `TimeoutPolicy` definitions:

```go
type RequestOptions struct {
	MaxAttempts   int
	Retry         RetryOptions
	TimeoutPolicy TimeoutPolicy
}

type requestOptionsContextKey struct{}

func WithRequestOptions(ctx context.Context, opts RequestOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestOptionsContextKey{}, cloneRequestOptions(opts))
}

func RequestOptionsFromContext(ctx context.Context) (RequestOptions, bool) {
	if ctx == nil {
		return RequestOptions{}, false
	}
	opts, ok := ctx.Value(requestOptionsContextKey{}).(RequestOptions)
	if !ok {
		return RequestOptions{}, false
	}
	return cloneRequestOptions(opts), true
}
```

- [ ] **Step 2: Add normalization helpers**

Add small unexported helpers so `execute` and `Capture` can share behavior. `cloneTimeoutPolicy` is required because both `RequestOptions` and `Options.TimeoutPolicy` contain maps:

```go
func cloneRequestOptions(opts RequestOptions) RequestOptions {
	opts.TimeoutPolicy = cloneTimeoutPolicy(opts.TimeoutPolicy)
	return opts
}

func cloneTimeoutPolicy(policy TimeoutPolicy) TimeoutPolicy {
	out := TimeoutPolicy{DefaultTimeout: policy.DefaultTimeout}
	if len(policy.OperationTimeouts) > 0 {
		out.OperationTimeouts = make(map[string]time.Duration, len(policy.OperationTimeouts))
		for operation, timeout := range policy.OperationTimeouts {
			out.OperationTimeouts[operation] = timeout
		}
	}
	if len(policy.MarketOperationTimeouts) > 0 {
		out.MarketOperationTimeouts = make(map[OperationMarket]time.Duration, len(policy.MarketOperationTimeouts))
		for operationMarket, timeout := range policy.MarketOperationTimeouts {
			out.MarketOperationTimeouts[operationMarket] = timeout
		}
	}
	return out
}

func shouldOverrideRetryOptions(opts RetryOptions) bool {
	return opts.Strategy != "" || opts.SameHostAttempts > 0
}

func hasTimeoutPolicyConfig(policy TimeoutPolicy) bool {
	if policy.DefaultTimeout > 0 {
		return true
	}
	for _, timeout := range policy.OperationTimeouts {
		if timeout > 0 {
			return true
		}
	}
	for _, timeout := range policy.MarketOperationTimeouts {
		if timeout > 0 {
			return true
		}
	}
	return false
}
```

Expected behavior:
- `MaxAttempts <= 0` means use the client's configured attempt count.
- empty `RetryOptions` means use the client's configured retry strategy.
- empty `TimeoutPolicy` means use the client's configured timeout policy.
- non-empty `TimeoutPolicy` overrides only non-zero default/operation/market entries; unspecified entries inherit the client's configured timeout policy.
- `NewClient` must also clone `Options.TimeoutPolicy`, so mutating the caller's original map later cannot affect an existing client.

- [ ] **Step 3: Run the focused package tests**

Run:

```bash
go test ./...
```

Expected: existing tests continue to pass before behavior is wired, because no caller uses `WithRequestOptions` yet.

### Task 2: Wire Request Options Into Attempts, Retry, And Timeout

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/client.go`
- Test: `/Users/liuhanqing01/projects/tdx/client_test.go`

- [ ] **Step 1: Create a resolved request config**

Add an internal struct and resolver:

```go
type requestPolicy struct {
	attempts      int
	retry         RetryOptions
	timeoutPolicy TimeoutPolicy
}

func (c *Client) requestPolicy(ctx context.Context) requestPolicy {
	policy := requestPolicy{
		attempts: c.attempts,
		retry: RetryOptions{
			Strategy:         c.retryStrategy,
			SameHostAttempts: c.sameHostAttempts,
		},
		timeoutPolicy: c.opts.TimeoutPolicy,
	}
	if opts, ok := RequestOptionsFromContext(ctx); ok {
		if opts.MaxAttempts > 0 {
			policy.attempts = opts.MaxAttempts
		}
		if shouldOverrideRetryOptions(opts.Retry) {
			policy.retry = normalizeRetryOptions(opts.Retry)
		}
		if hasTimeoutPolicyConfig(opts.TimeoutPolicy) {
			policy.timeoutPolicy = mergeTimeoutPolicy(policy.timeoutPolicy, opts.TimeoutPolicy)
		}
	}
	if policy.attempts <= 0 {
		policy.attempts = 1
	}
	policy.retry = normalizeRetryOptions(policy.retry)
	return policy
}
```

- [ ] **Step 2: Make retry picking use resolved options**

Change `pickAttemptServer` to accept a resolved retry config instead of reading `c.retryStrategy` directly:

```go
func (c *Client) pickAttemptServer(op string, plan *retryAttemptPlan, retry RetryOptions) (int, model.Server) {
	if retry.Strategy == RetryStrategySameHostFirst && retry.SameHostAttempts > 1 {
		// existing same-host-first logic
	}
	return c.pickServer(op)
}
```

Update both `execute` and `Capture` to call `policy := c.requestPolicy(ctx)` once before the loop, use `policy.attempts`, and pass `policy.retry` to `pickAttemptServer`.

- [ ] **Step 3: Make command timeout use resolved timeout policy**

Change `contextForCommand` to accept the resolved policy:

```go
func contextForCommand(ctx context.Context, cmd command.Command, policy TimeoutPolicy) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := policy.TimeoutFor(cmd)
	// keep existing deadline-preserving behavior
}
```

Update both `execute` and `Capture` call sites.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./... 
```

Expected: all existing tests pass.

### Task 3: Regression Tests

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/client_test.go`

- [ ] **Step 1: Test context MaxAttempts override**

Add a test that creates a client with `MaxAttempts: 1`, two fake hosts where the first fails and second succeeds, then calls:

```go
ctx := WithRequestOptions(context.Background(), RequestOptions{MaxAttempts: 2})
count, err := client.GetSecurityCount(ctx, model.MarketSH)
```

Expected:
- `err == nil`
- `count == 100`
- observed hosts are `bad,good`

- [ ] **Step 2: Test context retry strategy override**

Add a test that creates a client with default failover-first, two fake hosts where the first host is permanently bad, then calls:

```go
ctx := WithRequestOptions(context.Background(), RequestOptions{
	Retry: RetryOptions{Strategy: RetryStrategySameHostFirst, SameHostAttempts: 2},
})
_, err := client.GetSecurityCount(ctx, model.MarketSH)
```

Expected:
- `err != nil`
- observed hosts are `bad,bad`
- this proves request context overrode the client default strategy.

- [ ] **Step 3: Test context timeout policy override**

Add a test with a client default timeout policy of `security_count: 3s` and `security_list: 2s`, then pass:

```go
ctx := WithRequestOptions(context.Background(), RequestOptions{
	TimeoutPolicy: TimeoutPolicy{
		OperationTimeouts: map[string]time.Duration{
			"security_count": 1200 * time.Millisecond,
		},
	},
})
```

The fake round tripper should inspect `ctx.Deadline()` and assert `security_count` is around `1200ms`, while `security_list` still inherits the client's `2s` timeout.

- [ ] **Step 4: Test edge cases**

Add tests for:

- nil context plus timeout policy does not panic and still applies a deadline.
- zero `RequestOptions{}` does not override client policy.
- invalid request retry strategy normalizes to failover-first and does not mutate the client's retry fields.
- `WithRequestOptions`, `RequestOptionsFromContext`, and `NewClient` clone timeout policy maps.

- [ ] **Step 5: Run focused regression tests**

Run:

```bash
go test ./... 
```

Expected: new tests pass with existing tests.

### Task 4: Documentation

**Files:**
- Modify: `/Users/liuhanqing01/projects/tdx/README.md`
- Modify: `/Users/liuhanqing01/projects/tdx/docs/api/protocol-interface-map.md`

- [ ] **Step 1: Document the policy layering**

Add a short paragraph to README's timeout/retry section:

```markdown
请求级临时覆盖使用 `tdx.WithRequestOptions(ctx, opts)`，不要把 retry 参数塞进每个数据接口。推荐分层是：`tdx.Options` 放生产默认值，`WithRequestOptions` 处理少量请求的临时 SLA，`XxxWithOptions` 只处理分页、市场、chunk 等业务预算。
```

- [ ] **Step 2: Add a compact example**

Add this example near the retry policy section:

```go
ctx = tdx.WithRequestOptions(ctx, tdx.RequestOptions{
    MaxAttempts: 1,
    TimeoutPolicy: tdx.TimeoutPolicy{
        OperationTimeouts: map[string]time.Duration{
            "security_list": 800 * time.Millisecond,
        },
    },
})
list, err := client.ListSecurities(ctx, model.MarketBJ)
```

- [ ] **Step 3: Update protocol interface map**

Add a note to the composite API section:

```markdown
`WithRequestOptions(ctx, opts)` is transport policy, not a TDX command. It affects retries, attempts, and command deadlines for the request chain carried by that context.
```

Mention that request timeout maps are defensively copied when saved to context, and client timeout maps are copied by `NewClient`.

### Task 5: Verification And Commit

**Files:**
- Verify all modified files.

- [ ] **Step 1: Format**

Run:

```bash
gofmt -w client.go client_test.go
```

- [ ] **Step 2: Test**

Run:

```bash
go test -count=1 ./...
go vet ./...
go test -race .
go test -run=^$ -bench=Client -benchmem .
```

Expected:
- all tests pass
- vet has no findings
- race test for the root package passes
- benchmark still runs

- [ ] **Step 3: Commit**

Run:

```bash
git add client.go client_test.go README.md docs/api/protocol-interface-map.md docs/superpowers/plans/2026-06-11-request-options.md
git commit -m "feat: add request-level policy overrides"
```

Expected: commit succeeds and `.idea/` remains untracked.

## Self-Review

- Spec coverage: covers public helper API, request-level max attempts, retry strategy, timeout policy, tests, docs, verification, and commit.
- Placeholder scan: no TBD/TODO/fill-in-later instructions.
- Type consistency: `RequestOptions`, `WithRequestOptions`, `RequestOptionsFromContext`, `RetryOptions`, and `TimeoutPolicy` match existing project naming.
