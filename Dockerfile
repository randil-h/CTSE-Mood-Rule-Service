# Multi-stage Dockerfile for Mood-Rule-Service
# Stage 1: Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make protobuf-dev protobuf

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY . .

# Install protoc-gen-go and protoc-gen-go-grpc (pinned versions for Go 1.22 compatibility)
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# Generate protobuf files in correct subdirectories
RUN mkdir -p proto/moodrule proto/auth proto/productcatalog && \
    protoc --go_out=. --go_opt=module=github.com/randil-h/CTSE-Mood-Rule-Service \
    --go-grpc_out=. --go-grpc_opt=module=github.com/randil-h/CTSE-Mood-Rule-Service \
    proto/mood_rule_service.proto proto/auth_service.proto proto/product_catalog.proto

# Build the application with GOFLAGS=-mod=mod to allow go.sum updates
RUN mkdir -p /app/bin && \
    GOFLAGS=-mod=mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/bin/mood-rule-service ./cmd/server

# Stage 2: Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/mood-rule-service /app/mood-rule-service

# Copy migrations (optional, if you want to run them from container)
COPY --from=builder /app/migrations /app/migrations

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 50051 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the application
ENTRYPOINT ["/app/mood-rule-service"]
