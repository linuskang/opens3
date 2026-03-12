# ─── Stage 1: Build React UI ─────────────────────────────────────────────────
FROM node:24-alpine AS ui-builder

WORKDIR /build/ui

COPY cmd/opens3/ui/package*.json ./
RUN npm ci --prefer-offline

COPY cmd/opens3/ui/ ./
RUN npm run build

# ─── Stage 2: Build Go binary ────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder

WORKDIR /build

# Copy Go modules first for cache efficiency.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Embed the pre-built UI.
COPY --from=ui-builder /build/ui/dist ./cmd/opens3/ui/dist

RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -o /opens3 ./cmd/opens3/

# ─── Stage 3: Minimal runtime image ─────────────────────────────────────────
FROM alpine:3.21

LABEL org.opencontainers.image.title="Opens3"
LABEL org.opencontainers.image.description="Lightweight S3-compatible object storage with web UI"
LABEL org.opencontainers.image.source="https://github.com/linuskang/opens3"

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S opens3 && \
    adduser -S -G opens3 opens3

WORKDIR /app
COPY --from=go-builder /opens3 ./opens3

RUN mkdir -p /data && chown opens3:opens3 /data

USER opens3

VOLUME ["/data"]
EXPOSE 9000 9001

ENV OPENS3_UI_PORT=9000 \
    OPENS3_API_PORT=9001 \
    OPENS3_DATA_DIR=/data \
    OPENS3_ACCESS_KEY=minioadmin \
    OPENS3_SECRET_KEY=minioadmin \
    OPENS3_REGION=us-east-1

ENTRYPOINT ["/app/opens3"]
