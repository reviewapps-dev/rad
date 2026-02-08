#!/usr/bin/env bash
set -euo pipefail

# Test teardown script — removes a deployed app
# Usage: ./scripts/test-teardown.sh [app_id]

TOKEN="${RAD_TOKEN:-secret}"
HOST="${RAD_HOST:-localhost:7890}"
APP_ID="${1:-api-with-versioning}"

echo "==> Tearing down ${APP_ID}"

curl -s -X DELETE "http://${HOST}/apps/${APP_ID}" \
  -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool

echo ""
echo "==> Verifying removal"
curl -s "http://${HOST}/apps" \
  -H "Authorization: Bearer ${TOKEN}" | python3 -m json.tool
