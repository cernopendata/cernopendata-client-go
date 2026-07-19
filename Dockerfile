# Build stage
FROM golang:1.26-alpine AS builder

ARG VERSION=dev

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with the version supplied by the release or container workflow. The
# build context intentionally excludes .git, so version discovery cannot happen
# reliably inside the image build.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.buildVersion=${VERSION}" -o cernopendata-client ./cmd/cernopendata-client

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
# ca-certificates: for TLS connections
RUN apk add --no-cache ca-certificates

# Copy the binary
COPY --from=builder /app/cernopendata-client /usr/local/bin/cernopendata-client

# Set entrypoint
ENTRYPOINT ["cernopendata-client"]
