#!/usr/bin/env bash
# Smoke-test the cavimg-mcp stdio protocol without a container.
# Pipes initialize + initialized + tools/list, asserts all four tools appear.
#
# stdin is held open briefly after the requests: a stdio MCP server tears the
# connection down on read-EOF, which can race ahead of flushing in-flight
# responses. Real clients keep stdin open for the whole session; the trailing
# sleep simulates that so responses are captured deterministically.
set -euo pipefail

cd "$(dirname "$0")/.."

ext=""
if [ "$(go env GOOS)" = "windows" ]; then ext=".exe"; fi
bin="cavimg-mcp${ext}"

go build -o "$bin" .

req=$(cat <<'JSON'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
JSON
)

out=$({ printf '%s\n' "$req"; sleep 1; } | "./$bin")

fail=0
for tool in detect_project install_cavimg list_image_usages apply_cavimg; do
  if ! grep -q "\"$tool\"" <<<"$out"; then
    echo "MISSING tool: $tool"
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo "SMOKE FAILED"
  echo "$out"
  exit 1
fi
echo "SMOKE OK: all four tools listed"
