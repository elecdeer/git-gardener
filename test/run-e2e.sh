#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Building Docker test image..."
docker build -t git-gardener-test -f "$SCRIPT_DIR/Dockerfile" "$PROJECT_ROOT"

echo "Running e2e tests in Docker container..."
docker run --rm git-gardener-test
