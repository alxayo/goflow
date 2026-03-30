# goflow

An AI workflow orchestration engine for multi-agent LLM workflows, powered by the Copilot SDK.

## Current Implementation Notes

The repository includes roadmap-oriented examples and docs, but the current CLI behavior is narrower than the full design surface in some areas.

The most important current facts are:

1. `goflow run` executes the parallel orchestrator (`RunParallel`) level by level.
2. `config.max_concurrency` is active and limits concurrent steps within each parallel DAG level.
3. In parallel levels (fan-out), step failures are handled with best effort: sibling steps continue and downstream fan-in steps can still run using empty output for failed dependencies.
4. `retry_count` is active for timeout-style transient failures in step session creation/send, with short linear backoff between retries.
5. **Event-based session monitoring**: Sessions complete naturally when the LLM finishes (via `session.idle` event). No timeout configuration is required for long-running operations.
6. Step `timeout` is **optional** — use it only as a safety limit for CI/CD or to prevent runaway sessions. Most workflows don't need it.
7. `--verbose` mode shows real-time progress: tool calls, agent delegations, and session completion.
8. `--stream` mode shows the LLM's response as it generates, token by token.
9. `output.truncate` is parsed and helper code exists, but normal workflow execution does not automatically apply truncation yet.
10. Shared-memory helpers exist in the codebase, but automatic shared-memory wiring is not yet active in the main CLI flow.

For the implementation-accurate field-by-field reference, see [SETTINGS_REFERENCE.md](SETTINGS_REFERENCE.md) and [DOCS.md](DOCS.md).

## Run an Example Workflow

This project includes example workflow files in the examples folder.

### 1. Run the sequential example

From the repository root:

```bash
goflow run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/workflow/*.go' \
	--verbose
```

Note: Relative agent paths in workflow files (like `../agents/security-reviewer.agent.md`) resolve relative to the workflow file's location, so you can run the command from any directory.

### 2. Examples for specific folders

Review only workflow package files:

```bash
goflow run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/workflow/*.go' \
	--verbose
```

Review only executor package files:

```bash
goflow run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/executor/*.go' \
	--verbose
```

Review all Go files in the repository:

```bash
goflow run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='**/*.go' \
	--verbose
```

### 3. Real run vs mock run

- Real run (default): uses the Copilot SDK executor (which manages Copilot CLI automatically) to generate actual review content. Pass `--cli` to use the legacy CLI subprocess executor instead.
- Mock run: add `--mock` for deterministic test output.
- Interactive run: add `--interactive` to let agents ask clarification questions in the terminal.

Mock example:

```bash
goflow run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/workflow/*.go' \
	--mock \
	--verbose
```

### 4. Find run artifacts

Each run writes audit artifacts under the workflow's audit directory (defaults to `.workflow-runs` in the current working directory).

```bash
ls -1 .workflow-runs | tail -n 5
```

Step outputs are stored in:

- examples/.workflow-runs/<run-id>/steps/00_security-review/output.md
- examples/.workflow-runs/<run-id>/steps/01_perf-review/output.md
- examples/.workflow-runs/<run-id>/steps/02_summary/output.md

### 5. Show build version

Release builds expose embedded build metadata:

```bash
goflow version
```

The output includes the semantic version tag, short commit SHA, and build timestamp.

## Build From Source

```bash
go build -o goflow ./cmd/workflow-runner/main.go
```

## Implemented CLI Commands

The current CLI implements:

- `goflow run`
- `goflow version`
- `goflow help`

Older docs may mention `goflow validate` or `goflow list`, but those commands are not currently implemented in `cmd/workflow-runner/main.go`.

## Releases

The repository includes a manual GitHub Actions release workflow at [.github/workflows/release.yml](.github/workflows/release.yml).

From the Actions tab, run `Release` and provide a semantic version without the leading `v`, for example `1.2.3` or `1.2.3-rc.1`. The workflow will:

- validate the version format and fail if the Git tag already exists
- run `go test ./...`
- build release archives for Linux (`amd64`, `arm64`), macOS (`amd64`, `arm64`), Windows (`amd64`), and Windows on Arm (`arm64`)
- create a GitHub Release tagged as `v<version>`
- upload platform archives plus a SHA-256 checksum file
- generate a Homebrew formula asset (`goflow.rb`) and Scoop manifest asset (`goflow.json`)
- prepend any operator notes you enter and append GitHub-generated release notes as the changelog

The repository also includes a dry-run validation workflow at [.github/workflows/release-validate.yml](.github/workflows/release-validate.yml). It runs on pull requests and can also be triggered manually to test the cross-platform packaging pipeline without creating a tag or publishing a release.

### Homebrew and Scoop

Each release produces two package manager metadata files:

- `goflow.rb`: a Homebrew formula that points to the macOS and Linux release archives and embeds the correct SHA-256 values for Intel and Arm builds.
- `goflow.json`: a Scoop manifest that points to the Windows `amd64` and `arm64` archives and embeds their SHA-256 values.

These files are always attached to the GitHub Release as assets. That gives you two operating modes:

1. **Metadata only:** download the generated files from the release and publish them yourself.
2. **Direct publishing from Actions:** let the workflow push them to a Homebrew tap and Scoop bucket automatically.

To enable direct publishing, configure these repository settings:

- Repository variable `HOMEBREW_TAP_REPOSITORY`: target tap repository, for example `your-org/homebrew-tap`
- Repository secret `HOMEBREW_TAP_TOKEN`: token with push access to that tap repository
- Repository variable `SCOOP_BUCKET_REPOSITORY`: target bucket repository, for example `your-org/scoop-bucket`
- Repository secret `SCOOP_BUCKET_TOKEN`: token with push access to that bucket repository

When you manually trigger `Release`, you can enable `publish_homebrew` and `publish_scoop`. If enabled and the corresponding repository variable and token are present, the workflow will:

- copy `goflow.rb` into `Formula/goflow.rb` in the Homebrew tap repository and push a commit
- copy `goflow.json` into `bucket/goflow.json` in the Scoop bucket repository and push a commit

That means end users can install with standard package manager commands once the tap or bucket is set up.
