#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
before=$(mktemp)
after=$(mktemp)
trap 'rm -f "$before" "$after"' EXIT HUP INT TERM

snapshot() {
  output=$1
  : > "$output"
  for directory in \
    "$root/apps/grafana-plugin/frontend/src/api/generated" \
    "$root/packages/generated-clients/go" \
    "$root/packages/generated-clients/typescript" \
    "$root/packages/generated-contracts/go" \
    "$root/packages/generated-contracts/typescript" \
    "$root/services/ai-core/internal/adapters/inbound/http/generated"
  do
    if [ -d "$directory" ]; then
      find "$directory" -type f ! -name '.gitkeep' -print | LC_ALL=C sort | while IFS= read -r file
      do
        shasum -a 256 "$file"
      done
    fi
  done > "$output"
}

snapshot "$before"
"$root/scripts/generate-clients.sh"
snapshot "$after"

if ! cmp -s "$before" "$after"; then
  diff -u "$before" "$after" >&2 || true
  printf '%s\n' 'generated output changed after regeneration' >&2
  exit 1
fi

printf '%s\n' 'generated client diff check passed'
