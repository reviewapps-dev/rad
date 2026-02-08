#!/usr/bin/env bash
set -euo pipefail

# Test health check — queries all endpoints
# Usage: ./scripts/test-health.sh

TOKEN="${RAD_TOKEN:-secret}"
HOST="${RAD_HOST:-localhost:7890}"

echo "==> Health"
curl -s "http://${HOST}/health" | python3 -m json.tool

echo ""
echo "==> Apps"
curl -s "http://${HOST}/apps" \
  -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool

echo ""
echo "==> Auth test (should fail)"
curl -s "http://${HOST}/apps" | python3 -m json.tool || true
