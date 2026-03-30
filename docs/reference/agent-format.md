# Agent File Format

This page documents the `.agent.md` format that the current loader parses, with a clear distinction between fields that affect runtime and fields that are only preserved for future or editor use.

---

## File Structure

An agent file has two parts:

1. YAML frontmatter between `---` markers
2. Markdown body used as the system prompt

Example:

```markdown
---
name: security-reviewer
description: Reviews code for vulnerabilities
tools:
  - grep
  - read_file
model: gpt-5
---

# Security Reviewer

You are an expert security reviewer...
```

---

## Fields That Affect Runtime Today

Parsed in `pkg/agents/loader.go`.

| Field | Exact behavior |
|---|---|
| `name` | Agent identity. If omitted, defaults to the file stem |
| `description` | Stored on the resolved agent |
| `tools` | Restricts the tool list passed to the executor when non-empty (SDK: `SessionConfig.AvailableTools`; CLI: `--available-tools` flag) |
| `model` | Accepts a string or list. Used as ordered model preference list |
| Markdown body | Becomes the session system prompt |

### `model` forms actually supported

```yaml
model: gpt-5
```

or

```yaml
model:
  - gpt-5
  - gpt-4o
```

The loader normalizes both into an ordered model list.

---

## Fields Parsed But Not Actively Used By The Current Executor

| Field | Current status |
|---|---|
| `agents` | Parsed and stored, but not used by the CLI runtime |
| `mcp-servers` | Parsed and stored, but not passed into the current step session config |
| `handoffs` | Parsed only |
| `hooks` | Parsed only |
| `argument-hint` | Parsed only |
| `user-invocable` | Parsed only |
| `disable-model-invocation` | Parsed only |
| `target` | Parsed only |

These fields are useful for compatibility and future expansion, but they should not be described as active runtime behavior in the current CLI.

---

## Tool Behavior

If `tools` is non-empty, the executor passes them as a restricted available-tools list. The SDK executor sets `SessionConfig.AvailableTools`; the CLI executor passes `--available-tools` on the command line.

If `tools` is empty or omitted, the executor allows all tools through the CLI default path.

That makes `tools` an allow-list, not a supplement.

---

## Claude Agent Compatibility

Files under `.claude/agents/` are normalized through a mapping layer so common Claude tool names are translated to the expected VS Code or Copilot equivalents.

This logic lives in `pkg/agents/loader.go`.

---

## Discovery Priority

Agent discovery priority is:

1. explicit workflow file references and inline agents
2. `.github/agents/`
3. `.claude/agents/`
4. `~/.copilot/agents/`
5. `config.agent_search_paths`

---

## Recommended Authoring Guidance

For the current runtime, prioritize these fields:

1. `name`
2. `description`
3. `tools`
4. `model`
5. strong markdown body instructions

Treat the rest as compatibility metadata unless you are extending the runtime.

---

## See Also

- [Settings And Options](settings-and-options.md)
- [Workflow Schema](workflow-schema.md)
