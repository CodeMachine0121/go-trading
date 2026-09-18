# One image, two programs. `migrate` and `server` are built from the same commit and
# shipped together on purpose: the schema and the code that depends on it should never
# be able to arrive separately. Which one runs is decided at launch — the deployment
# runs `migrate` first and `server` after — so there is no second image to keep in step.

FROM golang:1.26-alpine AS builder

WORKDIR /src

# Dependencies first, on their own layer, so editing Go files doesn't re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0: the PostgreSQL driver here is pure Go (pgx), so nothing needs a C
# library. Turning it off is what makes the binaries run on a base image that has no
# libc of its own to match.
# -trimpath drops the build machine's paths, -s -w drops debug symbols. Both only
# exist to make the image smaller.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.21

# ca-certificates: every market source this talks to is HTTPS — Binance, Fugle,
# Anthropic, Telegram. Without the root certificates the binary cannot verify any of
# them, and the failure reads as "x509: certificate signed by unknown authority",
# which looks like a problem with the other end rather than with this image.
#
# tzdata: "the session opens at 09:00" is a fact about Taipei, so the process has to
# be able to resolve Asia/Taipei (TAIWAN_STOCK_TIME_ZONE). Alpine ships no zone
# database, and the lookup fails silently into UTC — the market would appear to open
# at the wrong hour rather than to error.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 trading

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/migrate /usr/local/bin/migrate

USER trading

EXPOSE 8080

# For running the image by hand. Kubernetes ignores this and uses its own probes, so
# this exists for `docker run` — which is where the image gets checked before it ever
# reaches a cluster.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
