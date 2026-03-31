# VS Code Extension Bridge Mode

## Summary

Add a `--bridge` mode to goflow that enables it to serve as a backend for VS Code extensions (replacing sec-check's Python agent).

This unlocks:
- Using goflow as the backend for the sec-check VS Code extension
- Running any goflow workflow from VS Code with live progress UI
- Extensibility beyond security scanning

## Background

**Feasibility Status:** ✅ VERIFIED

Both sec-check (Python) and goflow (Go) use the Copilot SDK natively for LLM session management. The architecture is fundamentally compatible:

- Single shared SDK client managing one CLI connection
- Concurrent sessions spawned for parallel agent execution
- Language-specific concurrency (asyncio vs goroutines) but identical semantics
- JSON Lines protocol for bridging extension ↔ agent communication

The goflow `security-scan.yaml` workflow mirrors sec-check's 3-phase architecture:
```
discover → [bandit, guarddog, shellcheck, graudit, trivy] → aggregate → remediation
```

## Implementation Scope

This plan is **comprehensive and implementation-ready**. See [IMPLEMENTATION-PLAN-VSCODE-BRIDGE.md](IMPLEMENTATION-PLAN-VSCODE-BRIDGE.md) for full details.

### Phase 1: Bridge Protocol & Core Infrastructure
- Define JSON message types (commands, events, findings)
- Implement BridgeRunner (main loop: read commands, emit events)
- Wire progress callbacks to orchestrator
- Add `goflow bridge` CLI command

### Phase 2: Workflow Discovery & Listing
- Implement workflow discovery (find .yaml files in workspace)
- Handle `discover` and `list_workflows` commands
- Return workflow metadata (name, description, inputs)

### Phase 3: Findings Extraction & Parsing
- Parse structured findings from workflow Markdown output
- Support multiple finding formats (tables, lists, JSON blocks)
- Normalize severity levels

### Phase 4: Testing & Validation
- Unit tests for message protocols, discovery, findings parsing
- Integration tests for command flow and event sequences
- End-to-end bridge test with real security-scan workflow

### Phase 5: Documentation
- Bridge protocol specification (JSON Lines format)
- Bridge mode usage guide
- VS Code extension integration guide (for sec-check adaptation)

### Phase 6: (Out-of-scope) VS Code Extension Adaptation
- Information-only for sec-check team
- Replace Python spawn with goflow spawn
- Workflow selection UI (new capability)

## Technical Architecture

### Command Flow (Example: Security Scan)
```
Extension → {"type": "scan", "config": {"workflow": "security-scan.yaml"}}
              ↓
         BridgeRunner.handleScan()
              ↓
         Load workflow, merge inputs, create orchestrator
              ↓
         Orchestrator.RunParallel() with progress callbacks
              ↓
         Emit progress events: discovery → parallel_scan → synthesis → complete
              ↓
         Parse findings (Markdown → structured JSON)
              ↓
         Extension ← {"type": "result", "findings": [...]}
```

### Event Sequence (Simplified)
```json
{"type": "ready"}
{"type": "progress", "phase": "discovery"}
{"type": "progress", "phase": "parallel_scan", "scanners": [...]}
{"type": "progress", "phase": "synthesis"}
{"type": "result", "status": "success", "findings": [...]}
```

## Acceptance Criteria

- Bridge starts and emits `ready` event
- Scan command executes workflow with progress events
- Cancel command aborts cleanly
- Discover/list_workflows returns workflow metadata
- Findings extracted as structured JSON
- Unit, integration, and E2E tests passing
- Protocol and integration documentation complete
- Works with existing security-scan workflow and custom workflows

## Integration Points

- **Workflow compatibility:** No changes to existing workflows
- **SDK integration:** Uses existing CopilotSDKExecutor unchanged
- **CLI:** New subcommand `goflow bridge`, no breaking changes
- **Extension:** Protocol compatible with sec-check extension

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Protocol mismatch | Comprehensive protocol tests + example conversations |
| Finding parsing fragile | Support multiple formats + extensive test coverage |
| Performance regression | Benchmark event emission, optimize if needed |
| Discovery discovers non-workflows | Add schema validation + manual selection option |

## Deliverables

1. ✅ [IMPLEMENTATION-PLAN-VSCODE-BRIDGE.md](IMPLEMENTATION-PLAN-VSCODE-BRIDGE.md) — Comprehensive technical specification
2. 📋 Bridge protocol types (`pkg/bridge/types.go`)
3. 📋 Bridge runner (`pkg/bridge/runner.go`)
4. 📋 Discovery implementation (`pkg/bridge/discovery.go`)
5. 📋 Findings parser (`pkg/bridge/findings.go`)
6. 📋 CLI command (`cmd/workflow-runner/main.go` - modify)
7. 📋 Orchestrator callbacks (`pkg/orchestrator/orchestrator.go` - modify)
8. 📋 Unit & integration tests
9. 📋 Protocol documentation (`docs/reference/bridge-protocol.md`)
10. 📋 Usage guides (`docs/guide/`)

## Related Issues

- Security-scan workflow: [examples/security-scan/](https://github.com/alxayo/workflow-runner/tree/main/examples/security-scan)
- sec-check integration: https://github.com/alxayo/sec-check

## Next Steps

1. Review implementation plan
2. Assess timeline and resource availability
3. Create sub-issues for each phase if desired
4. Begin Phase 1 implementation
