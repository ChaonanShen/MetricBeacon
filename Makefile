SHELL := /bin/sh

.PHONY: bootstrap-check generate generated-client-diff validate-contracts lint test \
	test-adapters test-ai-core-domain test-sqlite test-assistant-mcp test-ai-mcp \
	test-ai-agent test-plugin-backend test-frontend test-diagnostics smoke e2e-mock e2e-real-metrics \
	diagnose-real-metrics diagnose-deepseek check boundary-check secret-scan

bootstrap-check:
	@./scripts/bootstrap-check.sh

generate:
	@./scripts/generate-clients.sh

generated-client-diff:
	@./scripts/generated-client-diff.sh

validate-contracts:
	@./scripts/validate-contracts.sh

lint:
	@test -z "$$(gofmt -l services/ai-core services/assistant-mcp apps/grafana-plugin/backend packages)"
	@cd apps/grafana-plugin/frontend && npm run typecheck

test: test-ai-core-domain test-sqlite test-assistant-mcp test-ai-mcp test-ai-agent test-plugin-backend test-frontend test-diagnostics

test-ai-core-domain:
	@cd services/ai-core && go test ./internal/domain/... ./internal/application/... ./internal/ports/...

test-sqlite:
	@cd services/ai-core && go test ./internal/adapters/outbound/storage/sqlite ./internal/adapters/outbound/events/inmemory

test-adapters: test-sqlite

test-assistant-mcp:
	@cd services/assistant-mcp && go test ./...

test-ai-mcp:
	@cd services/ai-core && go test ./internal/adapters/outbound/tools/mcp ./internal/adapters/inbound/http ./internal/application/workflows

test-ai-agent:
	@cd services/ai-core && go test ./internal/adapters/outbound/agent/... ./internal/bootstrap

test-plugin-backend:
	@cd apps/grafana-plugin/backend && GOPROXY=off go test ./...

test-frontend:
	@cd apps/grafana-plugin/frontend && npm run test && npm run typecheck

test-diagnostics:
	@node --test tests/diagnostics/*.test.mjs
	@sh -n scripts/wait-for-real-metrics.sh scripts/probe-real-prometheus.sh scripts/run-real-metrics-diagnostic.sh scripts/run-real-metrics-e2e.sh scripts/run-real-agent-e2e.sh

smoke: test-ai-mcp test-plugin-backend test-frontend

e2e-mock:
	@./scripts/run-mock-e2e.sh

e2e-real-metrics:
	@./scripts/run-real-metrics-e2e.sh

e2e-real-agent:
	@./scripts/run-real-agent-e2e.sh

diagnose-real-metrics:
	@./scripts/run-real-metrics-diagnostic.sh

diagnose-deepseek:
	@node tests/diagnostics/deepseek-smoke.mjs

boundary-check:
	@./scripts/check-boundaries.sh

secret-scan:
	@! rg -n --hidden --glob '!**/node_modules/**' --glob '!**/.git/**' '(AKIA[0-9A-Z]{16}|BEGIN (RSA|OPENSSH|EC) PRIVATE KEY)' .

check: generated-client-diff validate-contracts lint test boundary-check secret-scan
