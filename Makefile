SHELL := /bin/sh

.PHONY: bootstrap-check generate generated-client-diff validate-contracts lint test \
	test-adapters test-ai-core-domain test-sqlite test-assistant-mcp test-ai-mcp \
	test-plugin-backend test-frontend smoke e2e-mock check boundary-check secret-scan

bootstrap-check:
	@./scripts/bootstrap-check.sh

generate:
	@./scripts/generate-clients.sh

generated-client-diff:
	@./scripts/require-gate-implementation.sh "generated-client-diff requires G1 contract generation"

validate-contracts:
	@./scripts/validate-contracts.sh

lint:
	@./scripts/require-gate-implementation.sh "lint is introduced with G1 generated and source code"

test:
	@./scripts/require-gate-implementation.sh "test suites are introduced with their owning Gate"

test-adapters test-ai-core-domain test-sqlite test-assistant-mcp test-ai-mcp test-plugin-backend test-frontend smoke e2e-mock:
	@./scripts/require-gate-implementation.sh "$@ is not implemented before its owning Gate"

boundary-check:
	@./scripts/check-boundaries.sh

secret-scan:
	@./scripts/require-gate-implementation.sh "secret scan is introduced in G8"

check:
	@./scripts/require-gate-implementation.sh "check is assembled after G1-G8 validation targets exist"
