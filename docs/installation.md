# Installation

## Prerequisites

- Go 1.21+
- macOS, Linux, or WSL
- Copilot CLI for real (non-mock) runs

## Option 1: Build from source

```bash
git clone https://github.com/alxayo/goflow.git
cd goflow
go build -o goflow ./cmd/goflow
```

Verify:

```bash
./goflow version
```

## Option 2: Download release binaries

Use the Releases page and download the archive matching your OS/CPU:

- `goflow_<version>_linux_amd64.tar.gz`
- `goflow_<version>_linux_arm64.tar.gz`
- `goflow_<version>_darwin_amd64.tar.gz`
- `goflow_<version>_darwin_arm64.tar.gz`
- `goflow_<version>_windows_amd64.zip`
- `goflow_<version>_windows_arm64.zip`

Then extract and place `goflow` in your PATH.

## Option 3: Homebrew and Scoop

Release assets include generated package metadata files:

- `goflow.rb` (Homebrew formula)
- `goflow.json` (Scoop manifest)

If a tap or bucket is configured for your org, install using your standard package manager flow.

## Copilot CLI setup

Real runs call the Copilot CLI backend. Confirm availability:

```bash
which copilot
copilot --version
```

If unavailable, use `--mock` while developing workflow logic.

## Common install checks

```bash
# Confirm binary is executable
./goflow --help

# Run tests if you are developing locally
go test ./...
```
