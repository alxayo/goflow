# Advanced Usage

## Parallel orchestration behavior

goflow builds a DAG and executes it level by level.

- Steps in one level are dependency-safe to run together.
- Parallel mode uses goroutines and wait groups.
- Fan-in steps start only after all dependencies complete.
- In fan-out levels, failures are handled with best effort: siblings continue and failed outputs resolve to empty strings for fan-in.
- In single-step levels, failures are fail-fast.

## Retry and timeout behavior

Step-level retry is available through `retry_count`:

- Total attempts are `retry_count + 1`.
- Retries are limited to timeout-style transient failures.
- Backoff is short and linear between attempts.

The step-level `timeout` field is currently parsed but not yet enforced as a per-step execution deadline.

## Reliability patterns

1. Keep prompts deterministic and explicit.
2. Truncate large upstream outputs before reinjection.
3. Use aggregator steps to normalize varied agent output styles.
4. Run with `--mock` in CI for workflow shape validation.

## Advanced conditions

Use conditions for gating expensive or risky steps:

```yaml
condition:
  step: aggregate
  not_contains: "BLOCKER"
```

Or strict decisions:

```yaml
condition:
  step: release-gate
  equals: "APPROVE"
```

## Shared memory in parallel pipelines

Shared memory allows coordinated context in multi-agent fan-out branches.

```yaml
config:
  shared_memory:
    enabled: true
    inject_into_prompt: true
```

Enable only when cross-step context is required. Keep memory concise.

## Audit strategy for operations

Every run writes:

- workflow metadata
- input snapshot
- DAG artifact
- per-step prompt/output files
- errors and timings

Use run artifacts to debug regressions and compare behavior across versions.

## Performance tips

- Scope file globs tightly to reduce unnecessary context.
- Keep intermediate summaries short and factual.
- Prefer narrow specialist agents over one broad generalist prompt.
- Limit max concurrency when external tooling becomes a bottleneck.
