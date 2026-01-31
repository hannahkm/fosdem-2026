# Unified bpftrace to OpenTelemetry exporter
# Supports multiple modes: native-usdt, libstabst
#
# Build from project root:
#   docker build -f app/exporter/Dockerfile -t bpftrace-exporter .

ARG runtime_version=1.25.5
FROM golang:${runtime_version}-bookworm AS builder

WORKDIR /build

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the unified exporter
RUN CGO_ENABLED=0 go build -o bpftrace-exporter ./app/exporter/cmd/

# Final image with bpftrace
FROM quay.io/iovisor/bpftrace:latest

# Copy the exporter binary
COPY --from=builder /build/bpftrace-exporter /usr/local/bin/

# Copy all bpftrace scripts
COPY app/usdt/exporter/trace-json.bt /app/native-usdt.bt
COPY app/libstabst/exporter/trace-json.bt /app/libstabst.bt

# Set environment defaults
ENV OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
ENV TARGET_PID=1
ENV BPFTRACE_SCRIPT=/app/libstabst.bt
ENV EXPORTER_MODE=libstabst

ENTRYPOINT ["/usr/local/bin/bpftrace-exporter"]
