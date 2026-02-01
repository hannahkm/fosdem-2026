# Stage 1: Build Go from USDT fork
# This fork adds native USDT probe support to the Go runtime and stdlib
FROM debian:bookworm AS go-builder

RUN apt-get update && apt-get install -y \
    git \
    gcc \
    make \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install bootstrap Go (required to build Go from source)
# Go 1.24.6+ is required as the bootstrap compiler
ARG TARGETARCH
RUN curl -fsSL "https://go.dev/dl/go1.24.6.linux-${TARGETARCH}.tar.gz" | tar -C /usr/local -xz
ENV GOROOT_BOOTSTRAP=/usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Clone and build Go from the USDT fork
# Branch adds: runtime/trace/usdt package, stdlib instrumentation, go tool usdt
RUN git clone --branch poc_usdt --depth 1 \
    https://github.com/kakkoyun/go.git /go-src

WORKDIR /go-src/src
# The Go fork triggers a false-positive "stale targets" error in make.bash
# but the binaries ARE built successfully. See: https://github.com/golang/go/issues/70873
# We ignore the exit code and verify the binary exists instead.
ENV HOME=/root
RUN ./make.bash || true
RUN test -x /go-src/bin/go && /go-src/bin/go version

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

WORKDIR /app

RUN go mod init usdt

# Use the base app - stdlib is auto-instrumented with USDT probes
COPY app/main.go ./
COPY entry.sh ./

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./entry.sh"]
