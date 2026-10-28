#!/usr/bin/env bash
set -euo pipefail

echo "================================================================================"
echo "   PayFlow — Full System End-to-End Test Suite"
echo "================================================================================"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "▶ 1. Running Go Payment Service Tests..."
(cd "$ROOT_DIR/services/payment" && go test ./...)

echo "▶ 2. Running E2E Invariants Verification..."
(cd "$ROOT_DIR/services/payment" && go run ./cmd/e2e)

echo "▶ 3. Running API Gateway Tests..."
(cd "$ROOT_DIR/services/api-gateway" && npm test)

echo "▶ 4. Building React Web Dashboard..."
(cd "$ROOT_DIR/apps/web" && npm run build)

echo ""
echo "✅ All verification suites passed successfully!"
