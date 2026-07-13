#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

require_version() {
  actual=$1
  expected=$2
  name=$3
  if [ "$actual" != "$expected" ]; then
    printf '%s\n' "$name version mismatch: expected $expected, got $actual" >&2
    exit 1
  fi
}

require_version "$(go env GOVERSION)" "go1.26.5" "Go"
require_version "$(node --version)" "v22.23.1" "Node.js"
require_version "$(npm --version)" "10.9.8" "npm"

for module in \
  apps/grafana-plugin/backend \
  services/ai-core \
  services/assistant-mcp \
  packages/request-context-go \
  packages/testkit-go
do
  # go test compiles every package without leaving command binaries in the module root.
  (cd "$root/$module" && go test ./...)
done

node -e 'JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"))' \
  "$root/apps/grafana-plugin/plugin.json"

if [ ! -x "$root/apps/grafana-plugin/frontend/node_modules/.bin/tsc" ]; then
  printf '%s\n' "frontend dependencies are absent; run: cd apps/grafana-plugin/frontend && npm ci" >&2
  exit 1
fi
(cd "$root/apps/grafana-plugin/frontend" && npm run typecheck)

"$root/scripts/check-boundaries.sh"
printf '%s\n' "bootstrap-check passed"
