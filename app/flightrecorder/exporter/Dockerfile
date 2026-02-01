# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Copy go mod files first for caching
COPY go.mod ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the exporter
RUN CGO_ENABLED=0 GOOS=linux go build -o flightrecorder-exporter .

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/flightrecorder-exporter .

# Create trace directory
RUN mkdir -p /tmp/traces

ENV OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
ENV TRACE_OUTPUT_DIR=/tmp/traces

ENTRYPOINT ["./flightrecorder-exporter"]
