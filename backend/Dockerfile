# syntax=docker/dockerfile:1
# Build context: ./backend
FROM golang:1.27-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -trimpath -o /out/api ./cmd/api
# Directories the API writes to; owned by the unprivileged runtime user.
RUN mkdir -p /out/data/storage /out/data/uploads && chown -R 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/api /api
COPY --from=builder --chown=65532:65532 /out/data /data
ENV APP_ENV=production \
    APP_PORT=8080 \
    STORAGE_PROVIDER=local \
    LOCAL_STORAGE_PATH=/data/storage \
    UPLOAD_TMP_DIR=/data/uploads
USER 65532:65532
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD ["/api", "healthcheck"]
ENTRYPOINT ["/api"]
