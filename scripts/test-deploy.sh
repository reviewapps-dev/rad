#!/usr/bin/env bash
set -euo pipefail

# Test deploy script — sends a deploy request to the rad daemon
# Usage: ./scripts/test-deploy.sh [repo_url] [branch]

TOKEN="${RAD_TOKEN:-secret}"
HOST="${RAD_HOST:-localhost:7890}"
APP_ID="${APP_ID:-api-with-versioning}"
REPO_URL="${1:-https://github.com/afomera/api_with_versioning.git}"
BRANCH="${2:-main}"

echo "==> Deploying ${APP_ID} from ${REPO_URL} (branch: ${BRANCH})"

curl -s -X POST "http://${HOST}/apps/deploy" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"app_id\": \"${APP_ID}\",
    \"repo_url\": \"${REPO_URL}\",
    \"branch\": \"${BRANCH}\",
    \"subdomain\": \"${APP_ID}\",
    \"callback_url\": \"http://localhost:3000/api/v1/builds/1/status\"
  }" | python3 -m json.tool

echo ""
echo "==> Deploy queued. Monitor with:"
echo "    curl -s http://${HOST}/apps/${APP_ID}/status -H 'Authorization: Bearer ${TOKEN}' | python3 -m json.tool"
