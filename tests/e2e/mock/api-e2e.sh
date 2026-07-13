#!/bin/sh
set -eu
exec node "$(dirname "$0")/api-e2e.mjs"
