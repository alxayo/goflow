# Installation

This page covers everything you need to install and run goflow.

---

## Prerequisites

Before installing goflow, make sure you have:

| Requirement | Why You Need It | How to Check |
|-------------|-----------------|--------------|
| **Go 1.21+** | goflow is written in Go | `go version` |
| **macOS, Linux, or WSL** | Supported operating systems | — |
| **Copilot CLI** (optional) | Required for real AI calls, not needed for `--mock` mode | `which copilot` |

!!! tip "Don't have Copilot CLI?"
    You can still learn and test goflow using `--mock` mode, which simulates AI responses without making real API calls.

---

## Option 1: Build from Source (Recommended)

This is the fastest way to get started if you already have Go installed.

### Step 1: Clone the repository

```bash
git clone https://github.com/alxayo/goflow.git
cd goflow
```

### Step 2: Build the binary

```bash
go build -o goflow ./cmd/goflow
```

This creates a `goflow` executable in your current directory.

### Step 3: Verify installation

```bash
./goflow version
```

You should see version information with the build date and commit hash.

### Step 4 (Optional): Add to PATH

To run `goflow` from anywhere:

```bash
# Move to a directory in your PATH
sudo mv goflow /usr/local/bin/

# Or add the current directory to your PATH
export PATH="$PATH:$(pwd)"
```

---

## Option 2: Download Pre-built Binaries

Visit the [Releases page](https://github.com/alxayo/goflow/releases) and download the archive for your system:

| Operating System | Architecture | File |
|------------------|--------------|------|
| macOS | Intel | `goflow_VERSION_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `goflow_VERSION_darwin_arm64.tar.gz` |
| Linux | x64 | `goflow_VERSION_linux_amd64.tar.gz` |
| Linux | ARM64 | `goflow_VERSION_linux_arm64.tar.gz` |
| Windows | x64 | `goflow_VERSION_windows_amd64.zip` |
| Windows | ARM64 | `goflow_VERSION_windows_arm64.zip` |

### Extract and install

=== "macOS/Linux"
    ```bash
    tar -xzf goflow_VERSION_darwin_arm64.tar.gz
    sudo mv goflow /usr/local/bin/
    ```

=== "Windows"
    Extract the zip file and add the folder to your PATH.

---

## Option 3: Homebrew (macOS/Linux)

If a Homebrew tap has been configured for your organization:

```bash
brew install your-org/tap/goflow
```

---

## Verify Your Installation

Run this command to confirm everything is working:

```bash
goflow version
```

**Expected output:**

```
goflow version v1.0.0 (abc1234) built 2026-03-15T10:30:00Z
```

---

## Setting Up Copilot CLI (For Real AI Calls)

goflow uses the Copilot CLI to communicate with AI models. If you want to run workflows with real AI responses (not mock mode), you need Copilot CLI installed.

### Check if Copilot CLI is installed

```bash
which copilot
# or
copilot --version
```

### If not installed

Follow the [Copilot CLI installation guide](https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line) from GitHub.

!!! note "Mock Mode Works Without Copilot CLI"
    If you're just learning goflow or testing workflow structures, you can use `--mock` mode which doesn't require Copilot CLI:
    ```bash
    goflow run --workflow my-workflow.yaml --mock --verbose
    ```

---

## Next Steps

Now that goflow is installed:

- :material-rocket-launch: [Quick Start](quickstart.md) — See goflow in action in 2 minutes
- :material-book-open-page-variant: [Your First Workflow](first-workflow.md) — Build a workflow step-by-step
