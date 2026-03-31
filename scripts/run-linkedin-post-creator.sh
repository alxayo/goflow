#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WORKFLOW="${REPO_ROOT}/examples/linkedin-post-creator/linkedin-post-creator.yaml"
BINARY="${REPO_ROOT}/goflow"

if [[ ! -f "${WORKFLOW}" ]]; then
  echo "Workflow not found: ${WORKFLOW}" >&2
  exit 1
fi

if [[ -x "${BINARY}" ]]; then
  exec "${BINARY}" run \
    --workflow "${WORKFLOW}" \
    --interactive \
    --verbose \
    --stream \
    "$@"
fi

exec go run ./cmd/workflow-runner run \
  --workflow "${WORKFLOW}" \
  --interactive \
  --verbose \
  --stream \
  "$@"
