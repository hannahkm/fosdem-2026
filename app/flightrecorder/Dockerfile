# Stage 1: Build Go from FlightRecorder fork
# This fork adds distributed tracing to Go's Flight Recorder
FROM golang:1.24-bookworm AS go-builder

RUN apt-get update && apt-get install -y \
    git \
    gcc \
    make \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Clone and build Go from the flight recorder fork
# Branch adds: event filtering, W3C Trace Context, specialized span types
RUN git clone --branch poc_flight_recorder --depth 1 \
    https://github.com/kakkoyun/go.git /go-src

WORKDIR /go-src/src
# Set bootstrap Go from the base image
ENV GOROOT_BOOTSTRAP=/usr/local/go
RUN ./make.bash

# Stage 2: Build application with custom Go
FROM debian:bookworm

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy custom Go toolchain
COPY --from=go-builder /go-src /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOROOT="/usr/local/go"

# Enable tracing via GODEBUG - selectively enable HTTP and network tracing
ENV GODEBUG="tracehttp=1,tracenet=1"

WORKDIR /app

RUN go mod init flightrecorder

# Use the base app - stdlib is auto-instrumented via GODEBUG
COPY app/main.go ./
COPY entry.sh ./

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./entry.sh"]
