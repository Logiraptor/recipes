#!/usr/bin/env bash
set -euo pipefail

MEALIE_BASE="${MEALIE_BASE:-https://mealie.home.poyarzun.io}"
MEALIE_TOKEN="${MEALIE_TOKEN:?MEALIE_TOKEN env var is required}"
JSON_DIR="${1:-./json}"

if [ ! -d "$JSON_DIR" ]; then
  echo "Error: directory '$JSON_DIR' does not exist" >&2
  exit 1
fi

shopt -s nullglob
files=("$JSON_DIR"/*.json)
if [ ${#files[@]} -eq 0 ]; then
  echo "No .json files found in $JSON_DIR"
  exit 0
fi

ok=0
fail=0

for f in "${files[@]}"; do
  name=$(jq -r '.name // "unknown"' "$f")

  # Ensure @context and @type are present for schema.org import
  recipe=$(jq '{
    "@context": "https://schema.org",
    "@type": "Recipe"
  } + .' "$f")

  payload=$(jq -n --arg data "$recipe" '{
    "includeTags": false,
    "data": $data
  }')

  http_code=$(curl -s -o /tmp/mealie-response.json -w '%{http_code}' \
    -X POST "${MEALIE_BASE}/api/recipes/create/html-or-json" \
    -H "Authorization: Bearer ${MEALIE_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "$payload")

  if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
    echo "OK   $name ($f)"
    ok=$((ok + 1))
  else
    echo "FAIL $name ($f) — HTTP $http_code"
    cat /tmp/mealie-response.json 2>/dev/null | jq . 2>/dev/null || cat /tmp/mealie-response.json 2>/dev/null
    echo
    fail=$((fail + 1))
  fi
done

echo
echo "Done: $ok succeeded, $fail failed (out of ${#files[@]} files)"
