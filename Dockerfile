FROM golang:1.23-alpine AS builder

WORKDIR /app

# Download dependencies first (cacheable layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
ARG VERSION=0.0.1
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X github.com/dk/varnish/internal/cli.Version=${VERSION}" \
    -o /app/varnish ./cmd/varnish

# Final image - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/varnish /varnish

USER nonroot:nonroot

ENTRYPOINT ["/varnish"]
