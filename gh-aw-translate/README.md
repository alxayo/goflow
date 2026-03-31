# gh-aw-translate

A standalone CLI tool that translates GitHub Agentic Workflow (gh-aw) `.md` files into [goflow](https://github.com/alex-workflow-runner) `.yaml` workflow definitions.

## Overview

GitHub Agentic Workflows define AI-powered automation using Markdown files with YAML frontmatter, running inside GitHub Actions. This tool converts those `.md` files into goflow YAML format so they can be executed locally, in any CI system, or anywhere the `goflow` CLI runs.

### What gets translated

- Markdown body → goflow step prompts
- Engine/model configuration → goflow config
- Tool declarations → agent tool lists
- MCP server definitions → agent MCP config
- Safe-outputs → prompt instructions (agent uses GitHub MCP server)
- Template expressions (`${{ }}`) → goflow input variables (`{{ }}`)
- Multi-workflow chains → single goflow DAG with `depends_on`

### What gets skipped (by design)

- **Triggers** (`on:`) — translated workflows run manually via `goflow run`
- **Security harness** — AWF firewall, threat detection, permission isolation
- **Platform-specific features** — rate limiting, cache-memory, repo-memory

See [SPEC-GH-AW-TRANSLATOR.md](../SPEC-GH-AW-TRANSLATOR.md) for the full specification.

## Installation

```bash
cd gh-aw-translate
go build -o gh-aw-translate ./cmd/gh-aw-translate/
```

## Usage

```bash
# Translate a single workflow
./gh-aw-translate --input .github/workflows/my-workflow.md --output ./translated/

# Translate a directory (auto-detect and merge connected workflows)
./gh-aw-translate --input .github/workflows/ --output ./translated/

# Dry-run: preview without writing files
./gh-aw-translate --input .github/workflows/ --output ./translated/ --dry-run
```

## Running Translated Workflows

```bash
# Set GitHub token for MCP server access
export GITHUB_TOKEN=ghp_...

# Run the translated workflow
goflow run --workflow translated/my-workflow.yaml --inputs repository=owner/repo
```

## Project Structure

```
gh-aw-translate/
├── cmd/gh-aw-translate/main.go    # CLI entry point
├── pkg/
│   ├── parser/                    # .md frontmatter + body splitting
│   ├── expression/                # ${{ }} scanning and rewriting
│   ├── mapper/                    # Tools, safe-outputs, engine, MCP mapping
│   ├── chain/                     # Multi-workflow chain detection and merging
│   ├── emitter/                   # goflow YAML + agent file generation
│   └── translator/                # Top-level orchestration
├── testdata/                      # Input fixtures and golden output files
├── go.mod
└── README.md
```

## License

Same as the parent goflow project.
