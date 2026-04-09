#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OUTPUT_NAME="${1:-goflow}"
OUTPUT_DIR="${2:-${REPO_ROOT}/bin}"
OUTPUT_PATH="${OUTPUT_DIR}/${OUTPUT_NAME}"

mkdir -p "${OUTPUT_DIR}"

cd "${REPO_ROOT}"
echo "Building goflow -> ${OUTPUT_PATH}"
go build -o "${OUTPUT_PATH}" ./cmd/workflow-runner/main.go

echo "Build complete: ${OUTPUT_PATH}"
