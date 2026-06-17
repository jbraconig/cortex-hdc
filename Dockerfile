# ── Stage 1: Build ─────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cortex ./cmd/cortex

# ── Stage 2: Runtime ───────────────────────────────────
FROM alpine:latest

# Certificates for HTTPS requests (webhook)
RUN apk --no-cache add ca-certificates

COPY --from=builder /cortex /usr/local/bin/cortex

WORKDIR /data

RUN mkdir -p /data/init-logs

ENTRYPOINT ["cortex"]
CMD ["auto", "--file", "/data/logs/syslog", "--verbose"]
