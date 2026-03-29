# goflow

An AI workflow orchestration engine that coordinates multi-agent LLM workflows with parallelism, powered by the Copilot SDK.

## Run an Example Workflow

This project includes example workflow files in the examples folder.

### 1. Run the sequential example

From the repository root:

```bash
go run ./cmd/workflow-runner run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/workflow/*.go' \
	--verbose
```

Note: Relative agent paths in workflow files (like `../agents/security-reviewer.agent.md`) resolve relative to the workflow file's location, so you can run the command from any directory.

### 2. Examples for specific folders

Review only workflow package files:

```bash
go run ./cmd/workflow-runner run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/workflow/*.go' \
	--verbose
```

Review only executor package files:

```bash
go run ./cmd/workflow-runner run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='pkg/executor/*.go' \
	--verbose
```

Review all Go files in the repository:

```bash
go run ./cmd/workflow-runner run \
	--workflow examples/simple-sequential.yaml \
	--inputs files='**/*.go' \
	--verbose
```

### 3. Real run vs mock run

- Real run (default): uses Copilot CLI to generate actual review content.
- Mock run: add `--mock` for deterministic test output.
- Interactive run: add `--interactive` to let agents ask clarification questions in the terminal.

Mock example:

```bash
go run ./cmd/workflow-runner run \
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

## Releases

The repository includes a manual GitHub Actions release workflow at [.github/workflows/release.yml](.github/workflows/release.yml).

From the Actions tab, run `Release` and provide a semantic version without the leading `v`, for example `1.2.3` or `1.2.3-rc.1`. The workflow will:

- validate the version format and fail if the Git tag already exists
- run `go test ./...`
- build release archives for Linux (`amd64`, `arm64`), macOS (`amd64`, `arm64`), Windows (`amd64`), and Windows on Arm (`arm64`)
- create a GitHub Release tagged as `v<version>`
- upload platform archives plus a SHA-256 checksum file
- prepend any operator notes you enter and append GitHub-generated release notes as the changelog
