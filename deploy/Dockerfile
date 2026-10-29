# Multi-stage build ultra-leger (compatible x86_64 et ARM64 pour Oracle Cloud)
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY pkg/ ./pkg/
COPY cmd/server/ ./cmd/server/

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /crazyping-server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /crazyping-server /usr/local/bin/crazyping-server

EXPOSE 4141/udp

ENTRYPOINT ["/usr/local/bin/crazyping-server"]
CMD ["-port", "4141", "-ttl", "3s"]
