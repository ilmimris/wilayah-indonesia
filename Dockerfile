# Multi-stage Dockerfile for Indonesian Regions Fuzzy Search API

# Stage 1: Builder
FROM golang:1.24 AS builder

# Install build tools
RUN apt-get update && apt-get install -y build-essential

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary as a fully static executable
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s -extldflags '-static'" -a -o regions-api ./cmd/api

# Stage 2: Final
FROM gcr.io/distroless/static-debian11

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/regions-api .

# Copy the database file
COPY --from=builder /app/data/regions.duckdb ./data/regions.duckdb

# Expose port
EXPOSE 8080

# Command to run the application
CMD ["/app/regions-api"]