#!/bin/bash
# run-security-scan.sh — Rebuild goflow and run the security-scan workflow
#
# Usage:
#   ./scripts/run-security-scan.sh [target_dir] [severity]
#
# Arguments:
#   target_dir  Directory to scan (default: current directory)
#   severity    Minimum severity: CRITICAL, HIGH, MEDIUM, LOW (default: MEDIUM)
#
# Examples:
#   ./scripts/run-security-scan.sh                    # Scan current directory
#   ./scripts/run-security-scan.sh ./src HIGH         # Scan ./src, HIGH+ severity
#   ./scripts/run-security-scan.sh . CRITICAL         # Only CRITICAL findings

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project root (script is in scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Parse arguments
TARGET_DIR="${1:-.}"
SEVERITY="${2:-MEDIUM}"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          goflow Security Scan Runner                       ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo

# Step 1: Build the application
echo -e "${YELLOW}▶ Building goflow...${NC}"
cd "$PROJECT_ROOT"

if go build -o goflow ./cmd/workflow-runner/main.go; then
    echo -e "${GREEN}✓ Build successful: ./goflow${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi
echo

# Step 2: Run the security scan workflow
echo -e "${YELLOW}▶ Running security-scan workflow...${NC}"
echo -e "  Target:   ${TARGET_DIR}"
echo -e "  Severity: ${SEVERITY}"
echo -e "  Flags:    --verbose --streaming"
echo

./goflow run \
    --workflow examples/security-scan/security-scan.yaml \
    --inputs "target=${TARGET_DIR}" \
    --inputs "severity=${SEVERITY}" \
    --verbose \
    --stream

echo
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Security scan complete!                           ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo
echo -e "Audit trail: ${BLUE}.workflow-runs/${NC}"
echo -e "Stream logs: ${BLUE}.workflow-runs/*/steps/*/stream.jsonl${NC}"
