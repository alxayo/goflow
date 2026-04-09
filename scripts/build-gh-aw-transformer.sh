#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

TARGET_PROJECT_DIR="${1:-${REPO_ROOT}/../gh-aw-transformer}"
OUTPUT_NAME="${2:-gh-aw-transformer}"
OUTPUT_DIR="${3:-${TARGET_PROJECT_DIR}/bin}"
OUTPUT_PATH="${OUTPUT_DIR}/${OUTPUT_NAME}"

if [[ ! -d "${TARGET_PROJECT_DIR}" ]]; then
  echo "Project directory not found: ${TARGET_PROJECT_DIR}" >&2
  exit 1
fi

if [[ ! -f "${TARGET_PROJECT_DIR}/go.mod" ]]; then
  echo "go.mod not found in: ${TARGET_PROJECT_DIR}" >&2
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"

cd "${TARGET_PROJECT_DIR}"
echo "Building gh-aw-transformer -> ${OUTPUT_PATH}"

if [[ -f "./cmd/gh-aw-transformer/main.go" ]]; then
  go build -o "${OUTPUT_PATH}" ./cmd/gh-aw-transformer/main.go
elif [[ -f "./cmd/main.go" ]]; then
  go build -o "${OUTPUT_PATH}" ./cmd/main.go
else
  go build -o "${OUTPUT_PATH}" .
fi

echo "Build complete: ${OUTPUT_PATH}"
