# Multi-stage Dockerfile for ggt transform service
# Build stage
FROM golang:1.24.0 AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Ensure configs directory exists
RUN mkdir -p /app/configs

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o ggt ./cmd/transform

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/ggt .

# Copy configs (optional, can be mounted as configmap)
COPY --from=builder /app/configs ./configs

# Change ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 9095 8085

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8085/healthz || exit 1

# Run the binary
CMD ["./ggt"]