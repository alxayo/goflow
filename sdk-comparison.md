# Copilot SDK vs LangChain vs Microsoft Agentic SDK: Primitives Comparison

A comparison of key agentic primitives across the three frameworks relevant to workflow-runner's execution backend.

---

## 1. Agent Files (`.agent.md`)

| Capability | Copilot SDK (current backend) | LangChain (`langchaingo`) | Microsoft Agent Framework (AutoGen → Semantic Kernel) |
|---|---|---|---|
| **Agent file format** | Native `.agent.md` with YAML frontmatter + markdown body | No concept — agents are code-defined only | No file format — code-defined `Agent()` / `AgentChat` / Kernel functions |
| **Agent discovery** | Auto-scans `.github/agents/`, `.claude/agents/`, `~/.copilot/agents/` | None | None |
| **Frontmatter fields** | Full VS Code spec: `name`, `description`, `tools`, `model`, `agents`, `mcp-servers`, `handoffs`, `hooks` | N/A | N/A |
| **Claude-format compat** | Yes — tool name mapping (`Read`→`view`, `Grep`→`grep`) | N/A | N/A |
| **Inline definitions** | Via workflow YAML `agents.*.inline` | Code-only | Code-only |
| **Handoffs metadata** | Parsed from frontmatter (`handoffs[].agent`, `.prompt`, `.send`) | No concept | AutoGen has agent routing; Semantic Kernel has no explicit handoffs |

**Gap summary:** `.agent.md` is a Copilot/VS Code-specific format. Neither LangChain nor Microsoft frameworks have anything equivalent — agent config is always in code. The workflow-runner bridges this via `pkg/agents/loader.go` which parses `.agent.md` and maps fields to `SessionConfig`, making the format backend-agnostic.

---

## 2. SKILL Files (`SKILL.md`)

| Capability | Copilot SDK | LangChain | Microsoft Agent Framework |
|---|---|---|---|
| **SKILL file support** | Native auto-discovery with `applyTo` glob matching | No concept | Semantic Kernel has a Plugin system (different paradigm) |
| **Discovery mechanism** | Scans `skills/` directories, matches `applyTo` patterns to runtime context | None | Plugins discovered via code registration |
| **Injection mechanism** | Auto-injected into agent's context window | N/A | Plugins are function-callable, not prompt-injected |
| **Format** | Markdown with YAML frontmatter (`applyTo`, `description`) | N/A | N/A |
| **Closest equivalent** | — | None (must manually concatenate into system prompt, ~30 lines) | CrewAI's "Knowledge" sources (Python only) |

**Gap summary:** SKILL files are a Copilot-only convention for structured domain-knowledge injection. LangChain has zero equivalent — but SKILLs are fundamentally just prompt injection. The content gets prepended/appended to the system prompt before sending to the LLM, so replicating this is trivial (~30 lines of code). Microsoft's Semantic Kernel has a richer Plugin system, but it's function-oriented (callable tools) not prompt-oriented (context injection).

---

## 3. MCP (Model Context Protocol) Servers

| Capability | Copilot SDK | LangChain | Microsoft Agent Framework |
|---|---|---|---|
| **Native MCP** | Yes — per-agent `mcp-servers:` in `.agent.md`, auto-wired | **No** — [open issue #1209](https://github.com/tmc/langchaingo/issues/1209) since April 2025, no timeline | AutoGen: Yes (`McpWorkbench` + `StdioServerParams`); Semantic Kernel: Yes (MCP as plugin type) |
| **Transport types** | stdio (via CLI) | N/A | AutoGen: stdio; OpenAI Agents SDK (for comparison): 4 transports (Hosted, HTTP, SSE, stdio) |
| **Server lifecycle** | Managed by CLI automatically | Must manage yourself | AutoGen manages via `McpWorkbench` |
| **Tool discovery** | Automatic — MCP tool schemas exposed to LLM | N/A | Automatic via `McpWorkbench.ListTools()` |
| **Tool filtering** | `tools: ["<server>/*"]` glob syntax | N/A | Supported in OpenAI Agents SDK |
| **Bridge effort** | 0 lines | ~150–200 lines via `mcp-go` + custom adapter | 0 lines (native) |

**Gap summary:** This is LangChain's biggest weakness. MCP is native in Copilot SDK and in Microsoft's AutoGen/Semantic Kernel, but completely absent from `langchaingo`. The workaround requires adding `mcp-go` as a dependency and writing ~150 lines of adapter code to convert MCP tool schemas to LangChain's `llms.Tool` format and manage server process lifecycle.

---

## 4. Built-in Tools

| Tool | Copilot SDK (via CLI) | LangChain (`langchaingo`) | Microsoft Semantic Kernel |
|---|---|---|---|
| `semantic_search` | Built-in (IDE-backed) | **None** — must build: embed + vector store (~200 lines + indexing) | Via plugins/embeddings |
| `grep` / `glob` | Built-in | **None** — must build (~30 lines each) | Via custom plugins |
| `view` / `read_file` | Built-in | **None** — must build (~20 lines) | Via custom plugins |
| `replace_string_in_file` | Built-in | **None** — must build (~40 lines) | Via custom plugins |
| `run_in_terminal` | Built-in | **None** — must build (~40 lines) | Via custom plugins |
| `fetch_webpage` | Built-in | ✅ `scraper` tool exists | Via custom plugins |
| Web search | Built-in | ✅ `serpapi`, `duckduckgo` | ✅ Bing plugin |
| SQL queries | Not built-in | ✅ `sqldatabase` | ✅ Database plugins |
| Calculator | Not built-in | ✅ `calculator` | ✅ Built-in |

**Gap summary:** Copilot CLI provides a complete filesystem/code toolset out of the box (6+ tools). LangChain ships **zero filesystem/code tools** — all must be hand-built. However, LangChain has richer data-access tools (SQL, web search, Wikipedia). Microsoft Semantic Kernel takes a plugin approach where you register native functions, OpenAPI endpoints, or MCP servers as tool sources.

---

## 5. Overall Primitives Summary

| Primitive | Copilot SDK | LangChain | Microsoft (AutoGen/SK) |
|---|---|---|---|
| **Agent files** | ✅ Native `.agent.md` | ❌ None | ❌ Code-only |
| **SKILL files** | ✅ Native auto-discovery | ❌ None (~30 lines to replicate) | ⚠️ Plugins (different paradigm) |
| **MCP servers** | ✅ Native | ❌ Missing (bridge ~150 lines) | ✅ Native |
| **Built-in code tools** | ✅ Rich (6+ tools) | ❌ None (must build ~250 lines) | ⚠️ Via plugins |
| **Hooks/callbacks** | ⚠️ 2 hooks (`onPreToolUse`, `onPostToolUse`) | ✅ Rich `callbacks.Handler` (10+ events) | ✅ Rich (filters, events) |
| **Multi-provider** | ⚠️ GitHub Models only (without BYOK) | ✅ 10+ providers natively | ✅ Multiple providers |
| **Local/offline models** | ❌ Requires internet + GitHub auth | ✅ Ollama, vLLM, llama.cpp | ✅ Ollama, ONNX, LMStudio |
| **Workflow/DAG** | ❌ None (you build it) | ❌ None (you build it) | ✅ Semantic Kernel Process Framework |
| **Guardrails** | ❌ None | ❌ None | ⚠️ OpenAI Agents SDK has them; SK/AutoGen don't |
| **Go support** | ✅ Native SDK | ✅ `langchaingo` | ❌ Python/C#/Java only |

---

## Key Takeaway

The workflow-runner's current architecture — Copilot SDK as default backend, with a `SessionExecutor` interface that enables swappable backends — is well-positioned. The Copilot SDK excels at the VS Code ecosystem primitives (agent files, SKILLs, MCP, built-in tools) but lacks multi-provider and offline support. LangChain fills that gap but requires ~430 lines of bridge code for MCP, tools, and SKILL injection. Microsoft's frameworks (AutoGen/Semantic Kernel) have the richest feature set but have no Go SDK, making them impractical as a direct backend.

For detailed implementation plans and bridge code estimates, see [research-langchain-support.md](research-langchain-support.md) (sections 2, 5, 6, and 11).
