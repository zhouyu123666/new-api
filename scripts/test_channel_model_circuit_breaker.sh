#!/usr/bin/env bash

set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
db_path=${NEW_API_DB_PATH:-"$repo_dir/one-api.db"}
base_url="http://127.0.0.1:3000"
model_name=${1:-gpt-5.6-luna}
request_count=${2:-10}

if ! [[ "$request_count" =~ ^[1-9][0-9]*$ ]]; then
  echo "request count must be a positive integer" >&2
  exit 1
fi

token_key=$(sqlite3 -cmd '.timeout 5000' "$db_path" "
  SELECT key FROM tokens
  WHERE status = 1 AND deleted_at IS NULL
  ORDER BY id LIMIT 1;
")
if [[ -z "$token_key" ]]; then
  echo "no active token found" >&2
  exit 1
fi

for ((attempt = 1; attempt <= request_count; attempt++)); do
  status=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 90 \
    -H "Authorization: Bearer $token_key" \
    -H 'Content-Type: application/json' \
    "$base_url/v1/chat/completions" \
    --data-binary "{\"model\":\"$model_name\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with exactly OK.\"}],\"max_tokens\":16,\"stream\":false}" || true)
  echo "attempt $attempt: HTTP $status"
done
