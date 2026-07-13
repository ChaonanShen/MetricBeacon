FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113 AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY packages/generated-clients/go ./packages/generated-clients/go
COPY packages/request-context-go ./packages/request-context-go
COPY services/ai-core ./services/ai-core
RUN mv go.work go.work.full && go work init ./packages/generated-clients/go ./packages/request-context-go ./services/ai-core && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ai-core ./services/ai-core/cmd/server

FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113
COPY --from=build /out/ai-core /usr/local/bin/ai-core
ENV AI_CORE_LISTEN_ADDRESS=:8080
ENV AI_CORE_SQLITE_PATH=/var/lib/ai-core/ai-core.sqlite
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=12 CMD curl -fsS http://127.0.0.1:8080/readyz || exit 1
ENTRYPOINT ["/usr/local/bin/ai-core"]
