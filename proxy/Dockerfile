# Self-contained build of tee-proxy. Clones tee-proxy from GitHub at build time
# so no local sibling repos are needed.
#
# TEE_PROXY_VERSION MUST stay aligned with the extension's Go-module pins
# (tools/go.mod): tee-proxy v0.0.18. tee-proxy's own go.mod transitively pins
# the matching tee-node and go-flare-common line, so building it with
# `go mod download` pulls the exact governance-aware node code the extension-tee
# image uses — no separate tee-node clone or replace directive is needed.
# Override via build arg when bumping both sides together.

ARG TEE_PROXY_VERSION=v0.0.18

FROM golang:1.25.1-alpine AS builder

RUN apk add --no-cache git

ARG TEE_PROXY_VERSION

WORKDIR /app
RUN git clone --depth 1 --branch ${TEE_PROXY_VERSION} https://github.com/flare-foundation/tee-proxy.git

WORKDIR /app/tee-proxy
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -o main ./cmd/proxy

# Final stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/tee-proxy/main .
COPY --from=builder /app/tee-proxy/config.example.toml ./config/config.toml

RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 6661
EXPOSE 6662

CMD ["./main"]
