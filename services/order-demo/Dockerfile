FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113 AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY packages/generated-clients/go ./packages/generated-clients/go
COPY services/order-demo ./services/order-demo
RUN mv go.work go.work.full && go work init ./packages/generated-clients/go ./services/order-demo && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/order-service ./services/order-demo/cmd/order-service && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/fault-controller ./services/order-demo/cmd/fault-controller && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/loadgen ./services/order-demo/cmd/loadgen

FROM golang@sha256:18aedc16aa19b3fd7ded7245fc14b109e054d65d22ed53c355c899582bbb2113
COPY --from=build /out/order-service /usr/local/bin/order-service
COPY --from=build /out/fault-controller /usr/local/bin/fault-controller
COPY --from=build /out/loadgen /usr/local/bin/loadgen
RUN mkdir -p /run/order-demo && chmod 0700 /run/order-demo
EXPOSE 8090 8091 8092 9102
ENTRYPOINT ["/usr/local/bin/order-service"]

