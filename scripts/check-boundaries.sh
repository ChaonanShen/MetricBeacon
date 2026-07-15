#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
core="$root/services/ai-core/internal"
order_demo="$root/services/order-demo/internal"

reject() {
  description=$1
  shift
  if rg -n --glob '*.go' --glob '!*_test.go' "$@"; then
    printf '%s\n' "dependency boundary violation: $description" >&2
    exit 1
  fi
}

# Domain, application and ports may depend on the standard library and project domain
# packages only. External SDKs belong in outbound/inbound adapters.
reject "AI Core domain/application/ports import an external SDK" \
  '"(github\.com|golang\.org|modernc\.org|go\.uber\.org)/' \
  "$core/domain" "$core/application" "$core/ports"
reject "AI Core domain imports application or adapters" \
  '"mini-torchbearing\.local/services/ai-core/internal/(application|adapters)/' \
  "$core/domain"
reject "AI Core application imports adapters" \
  '"mini-torchbearing\.local/services/ai-core/internal/adapters/' \
  "$core/application"

reject "order-demo domain/application/ports import an external SDK" \
  '"(github\.com|golang\.org|modernc\.org|go\.uber\.org)/' \
  "$order_demo/domain" "$order_demo/application" "$order_demo/ports"
reject "order-demo domain imports application or adapters" \
  '"mini-torchbearing\.local/services/order-demo/internal/(application|adapters)/' \
  "$order_demo/domain"
reject "order-demo application imports adapters" \
  '"mini-torchbearing\.local/services/order-demo/internal/adapters/' \
  "$order_demo/application"

# Fixtures are implementation details of the Mock Prometheus adapter, never an AI Core
# or MCP handler dependency.
reject "AI Core imports mock scenario fixtures" \
  'data/mock-scenarios|node_exporter_overview' \
  "$core/domain" "$core/application" "$core/ports"

printf '%s\n' "dependency boundary check passed"
