FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
COPY backend/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /api ./cmd/api

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /api /api
ENTRYPOINT ["/api"]
