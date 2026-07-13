#!/bin/sh
set -eu

exec "$(dirname -- "$0")/require-gate-implementation.sh" \
  "generate requires G1 shared OpenAPI and JSON Schema sources"
