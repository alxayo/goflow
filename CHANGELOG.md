# Changelog

All notable changes to goflow are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.2.0] — 2026-03-30

### Added

- **Stream recording to audit trail** (`stream.jsonl`): when `--streaming` is enabled, each step
  now records all LLM events (assistant turns, message deltas, tool executions, user input
  requests/responses) to `stream.jsonl` in JSON Lines format. Events are appended in real-time,
  enabling:
  - Live monitoring via `tail -f .workflow-runs/.../steps/*/stream.jsonl`
  - TUI stream switching between parallel steps
  - Interactive mode context: see the full stream before an LLM question
  - Full audit compliance with timestamped event trail
- **User input event recording**: interactive mode sessions now emit `user.input_requested` and
  `user.input_response` events to the stream, providing a complete audit trail of human-LLM
  interactions.
- **Copilot SDK executor** (`pkg/executor/copilot_sdk.go`): new default backend that drives
  the Copilot CLI via the Go SDK rather than a subprocess. One SDK session is created per
  workflow step for deterministic agent selection.
- **Event-based session monitoring** (`pkg/executor/monitor.go`): sessions now complete via
  `session.idle` event rather than a polling timeout, eliminating spurious timeout failures on
  long-running steps. The `SessionMonitor` tracks tool calls, streamed text, subagent
  delegations, and session errors in real time.
- **`--stream` CLI flag**: streams the LLM's response token-by-token to stderr as it generates,
  using `assistant.message_delta` events. Can be combined with `--verbose` for full visibility.
- **Retry logic for transient failures**: steps with `retry_count` now retry on
  context-deadline-exceeded and similar transient SDK errors with short linear backoff.
- **Verbose progress output** (`--verbose`): real-time lifecycle events for agent turns, tool
  calls, subagent delegations, and session completion/error.
- `SPEC-SDK-EXECUTOR.md`: detailed spec covering the SDK executor design, session lifecycle,
  BYOK provider support, and migration path from the CLI executor.
- `future-improvements.md`: tracks unimplemented SDK capabilities (session resume, per-step
  provider overrides, reasoning effort, OTel tracing, plan approval).
- `docs/reference/step-timeout.md`: new reference page explaining event-based completion and
  when step timeouts are (and are not) needed.
- `docs/reference/timeout-clarification.md`: FAQ-style companion clarifying common timeout
  misconceptions.

### Changed

- Default executor changed from CLI subprocess to Copilot SDK. Use `--cli` to fall back to the
  legacy subprocess executor.
- `--verbose` flag description updated: it now covers session lifecycle events (tool calls,
  subagent starts, session idle/error) rather than general logging.
- Parallel fan-out failure policy uses best-effort: sibling steps continue when one step in a
  parallel level fails; downstream fan-in steps may still run with partial inputs.
- Orchestrator wired to `RunParallel` by default (previously required explicit call).
- Event type names in `OnProgress` callbacks corrected to match SDK values
  (`assistant.turn_start`, `tool.execution_start`, `tool.execution_complete`).

### Fixed

- Numbered item duplicate in `README.md` implementation notes (item 9 appeared twice).

---

## [0.1.0] — 2026-03-29

Initial release.

### Added

- YAML workflow parser with full schema validation (`pkg/workflow/`)
- DAG builder with topological sort and cycle detection
- Template resolution for `{{inputs.X}}` and `{{steps.Y.output}}`
- Condition evaluation (`contains`, `not_contains`, `equals`)
- Agent loader and file-based discovery (`.agent.md`, Claude-format)
- CLI subprocess executor (fallback, `--cli`)
- Mock executor for deterministic local testing (`--mock`)
- Parallel orchestrator with configurable `max_concurrency`
- Audit trail: per-run timestamped directories with prompt, output, transcript, and metadata
- Reporter with `markdown`, `json`, and `plain` output formats
- Shared memory manager skeleton
- `goflow run`, `goflow version`, `goflow help` CLI commands
- Interactive mode (`--interactive`) for agents that ask clarification questions
- Example workflows: `simple-sequential`, `code-review-pipeline`, `security-scan`,
  `global-news-trends`, `decision-helper`, `guided-code-review`
- MkDocs documentation site
