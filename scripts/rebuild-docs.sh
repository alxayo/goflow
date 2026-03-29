#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VENV_DIR="${ROOT_DIR}/.venv-docs"
HOST="127.0.0.1"
PORT="8000"
OPEN_BROWSER="1"

usage() {
  cat <<'EOF'
Usage: scripts/rebuild-docs.sh [options]

Rebuilds the MkDocs site in strict mode and starts a local preview server.

Options:
  --port <number>   Preferred starting port (default: 8000)
  --venv <path>     Virtualenv path (default: .venv-docs)
  --no-open         Do not auto-open a browser tab
  -h, --help        Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)
      PORT="${2:-}"
      if [[ -z "$PORT" ]]; then
        echo "Error: --port requires a value" >&2
        exit 1
      fi
      shift 2
      ;;
    --venv)
      VENV_DIR="${2:-}"
      if [[ -z "$VENV_DIR" ]]; then
        echo "Error: --venv requires a value" >&2
        exit 1
      fi
      shift 2
      ;;
    --no-open)
      OPEN_BROWSER="0"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ ! -f "${ROOT_DIR}/mkdocs.yml" ]]; then
  echo "Error: mkdocs.yml not found in ${ROOT_DIR}" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "Error: python3 is required but not installed" >&2
  exit 1
fi

if [[ ! -d "$VENV_DIR" ]]; then
  echo "Creating virtualenv at ${VENV_DIR}..."
  python3 -m venv "$VENV_DIR"
fi

# shellcheck disable=SC1090
source "${VENV_DIR}/bin/activate"

if ! command -v mkdocs >/dev/null 2>&1; then
  echo "Installing docs dependencies..."
  pip install --upgrade pip >/dev/null
  pip install mkdocs mkdocs-material pymdown-extensions >/dev/null
fi

cd "$ROOT_DIR"

echo "Building docs (strict mode)..."
mkdocs build --strict

if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
  echo "Error: invalid port value: ${PORT}" >&2
  exit 1
fi

find_free_port() {
  local candidate="$1"
  while lsof -nP -iTCP:"${candidate}" -sTCP:LISTEN -t >/dev/null 2>&1; do
    candidate="$((candidate + 1))"
  done
  echo "$candidate"
}

PORT="$(find_free_port "$PORT")"

SITE_URL_RAW="$(grep -E '^site_url:' mkdocs.yml | head -n1 | sed -E 's/^site_url:[[:space:]]*//')"
SITE_URL_RAW="${SITE_URL_RAW%\"}"
SITE_URL_RAW="${SITE_URL_RAW#\"}"
SITE_URL_RAW="${SITE_URL_RAW%\'}"
SITE_URL_RAW="${SITE_URL_RAW#\'}"
SITE_PATH="$(echo "$SITE_URL_RAW" | sed -E 's#^https?://[^/]+##')"
if [[ -z "$SITE_PATH" ]]; then
  SITE_PATH="/"
fi
if [[ "${SITE_PATH:0:1}" != "/" ]]; then
  SITE_PATH="/${SITE_PATH}"
fi
if [[ "${SITE_PATH: -1}" != "/" ]]; then
  SITE_PATH="${SITE_PATH}/"
fi

PREVIEW_URL="http://${HOST}:${PORT}${SITE_PATH}"

if [[ "$OPEN_BROWSER" == "1" ]]; then
  if command -v open >/dev/null 2>&1; then
    open "$PREVIEW_URL" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$PREVIEW_URL" >/dev/null 2>&1 || true
  fi
fi

echo ""
echo "Docs preview ready"
echo "- URL: ${PREVIEW_URL}"
echo "- Press Ctrl+C to stop"
echo ""

exec mkdocs serve --dev-addr "${HOST}:${PORT}"
