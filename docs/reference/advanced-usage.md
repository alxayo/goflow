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

## Stream recording for debugging

With `--streaming` enabled, each step records all LLM events to `stream.jsonl`:

```bash
# Run with streaming enabled
goflow run --workflow review.yaml --streaming

# Tail a step's stream in real-time
tail -f .workflow-runs/.../steps/01_analyze/stream.jsonl
```

**Example stream.jsonl:**
```jsonl
{"ts":"2026-03-30T14:32:05.001Z","type":"assistant.turn_start"}
{"ts":"2026-03-30T14:32:05.050Z","type":"assistant.message_delta","data":"I'll analyze"}
{"ts":"2026-03-30T14:32:05.200Z","type":"tool.execution_start","data":{"tool":"grep"}}
{"ts":"2026-03-30T14:32:06.500Z","type":"tool.execution_complete","data":{"tool":"grep","status":"completed"}}
{"ts":"2026-03-30T14:32:07.100Z","type":"session.idle"}
```

This is useful for:

- **Debugging stuck steps**: See what the LLM was doing before a timeout
- **Interactive mode**: View accumulated context when LLM asks for user input
- **TUI development**: Switch between parallel step streams in real-time
- **Audit compliance**: Full transparency into LLM behavior

For interactive workflows, user input events are also recorded:

```jsonl
{"ts":"...","type":"user.input_requested","data":{"prompt":"Continue?","choices":["yes","no"]}}
{"ts":"...","type":"user.input_response","data":"yes"}
```

## Performance tips

- Scope file globs tightly to reduce unnecessary context.
- Keep intermediate summaries short and factual.
- Prefer narrow specialist agents over one broad generalist prompt.
- Limit max concurrency when external tooling becomes a bottleneck.
