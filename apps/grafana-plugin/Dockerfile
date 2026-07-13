FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113 AS backend-build
WORKDIR /src
COPY go.work go.work.sum ./
COPY packages/generated-clients/go/go.mod packages/generated-clients/go/go.sum ./packages/generated-clients/go/
COPY packages/request-context-go/go.mod ./packages/request-context-go/
COPY apps/grafana-plugin/backend/go.mod apps/grafana-plugin/backend/go.sum ./apps/grafana-plugin/backend/
RUN mv go.work go.work.full && go work init ./packages/generated-clients/go ./packages/request-context-go ./apps/grafana-plugin/backend
RUN --mount=type=cache,target=/go/pkg/mod \
    cd packages/generated-clients/go && go mod download \
    && cd /src/apps/grafana-plugin/backend && go mod download
COPY packages/generated-clients/go ./packages/generated-clients/go
COPY packages/request-context-go ./packages/request-context-go
COPY apps/grafana-plugin/backend ./apps/grafana-plugin/backend
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mini-torchbearing-app_linux_amd64 ./apps/grafana-plugin/backend/cmd/plugin

FROM grafana/grafana@sha256:121a7a9ece6dc10b969f1f96eed64b4f07dfac0d0b8abc070f7cb83bbde86f63
USER root
RUN mkdir -p /var/lib/grafana/plugins/mini-torchbearing-app
COPY apps/grafana-plugin/plugin.json /var/lib/grafana/plugins/mini-torchbearing-app/plugin.json
COPY apps/grafana-plugin/dist/ /var/lib/grafana/plugins/mini-torchbearing-app/
COPY apps/grafana-plugin/provisioning/mini-torchbearing.yaml /etc/grafana/provisioning/plugins/mini-torchbearing.yaml
COPY --from=backend-build /out/mini-torchbearing-app_linux_amd64 /var/lib/grafana/plugins/mini-torchbearing-app/mini-torchbearing-app_linux_amd64
RUN chmod +x /var/lib/grafana/plugins/mini-torchbearing-app/mini-torchbearing-app_linux_amd64 \
    && test -f /var/lib/grafana/plugins/mini-torchbearing-app/plugin.json \
    && test -f /var/lib/grafana/plugins/mini-torchbearing-app/module.js \
    && test ! -d /var/lib/grafana/plugins/mini-torchbearing-app/dist
USER grafana
