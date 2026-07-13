FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113 AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY packages/generated-contracts/go ./packages/generated-contracts/go
COPY packages/request-context-go ./packages/request-context-go
COPY services/assistant-mcp ./services/assistant-mcp
RUN mv go.work go.work.full && go work init ./packages/generated-contracts/go ./packages/request-context-go ./services/assistant-mcp && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/assistant-mcp ./services/assistant-mcp/cmd/server

FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113
COPY --from=build /out/assistant-mcp /usr/local/bin/assistant-mcp
COPY contracts/tools/grafana /app/schemas
COPY data/mock-scenarios/node_exporter_overview /app/fixture
ENV ASSISTANT_MCP_LISTEN_ADDRESS=:8081
ENV ASSISTANT_MCP_SCHEMA_DIR=/app/schemas
ENV ASSISTANT_MCP_FIXTURE_DIR=/app/fixture
EXPOSE 8081
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD curl -fsS http://127.0.0.1:8081/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/assistant-mcp"]
