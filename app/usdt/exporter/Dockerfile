FROM golang:1.23-bookworm AS builder

WORKDIR /build

RUN go mod init exporter

COPY exporter/main.go ./

RUN go mod tidy

RUN CGO_ENABLED=0 go build -o bpftrace-exporter .

# Final image with bpftrace
FROM quay.io/iovisor/bpftrace:latest

COPY --from=builder /build/bpftrace-exporter /usr/local/bin/

COPY exporter/trace-json.bt /app/trace-json.bt

ENV OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
ENV TARGET_PID=1
ENV BPFTRACE_SCRIPT=/app/trace-json.bt

ENTRYPOINT ["/usr/local/bin/bpftrace-exporter"]
