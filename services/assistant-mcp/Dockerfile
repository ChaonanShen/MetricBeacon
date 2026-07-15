FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113 AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY packages/approval-evidence-go ./packages/approval-evidence-go
COPY packages/generated-clients/go ./packages/generated-clients/go
COPY packages/generated-contracts/go ./packages/generated-contracts/go
COPY packages/request-context-go ./packages/request-context-go
COPY services/assistant-mcp ./services/assistant-mcp
RUN mv go.work go.work.full && go work init ./packages/approval-evidence-go ./packages/generated-clients/go ./packages/generated-contracts/go ./packages/request-context-go ./services/assistant-mcp && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/assistant-mcp ./services/assistant-mcp/cmd/server

FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113
COPY --from=build /out/assistant-mcp /usr/local/bin/assistant-mcp
COPY contracts/tools/grafana /app/schemas
COPY contracts/tools/incident /app/incident-schemas
COPY contracts/schemas/assets /app/asset-schemas
COPY data/mock-scenarios/node_exporter_overview /app/fixture
COPY data/operational-assets /app/operational-assets
ENV ASSISTANT_MCP_LISTEN_ADDRESS=:8081
ENV ASSISTANT_MCP_SCHEMA_DIR=/app/schemas
ENV ASSISTANT_MCP_FIXTURE_DIR=/app/fixture
ENV ASSISTANT_MCP_INCIDENT_SCHEMA_DIR=/app/incident-schemas
ENV ASSISTANT_MCP_ASSET_SCHEMA_DIR=/app/asset-schemas
ENV ASSISTANT_MCP_ASSET_DIR=/app/operational-assets
EXPOSE 8081
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD curl -fsS http://127.0.0.1:8081/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/assistant-mcp"]
