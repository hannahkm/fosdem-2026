# NOTE: salp library requires Go <= 1.23.x due to generics compatibility issues
# with newer Go versions (1.25+). Override runtime_version if needed.
ARG runtime_version=1.23.6
FROM golang:${runtime_version}-bookworm

# Install libstapsdt and dependencies
# Note: libstapsdt-dev may not be available in standard repos, so we build from source
RUN apt-get update && apt-get install -y \
    libelf-dev \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Build and install libstapsdt from source
# NOTE: libstapsdt uses memfd_create + dlopen("/proc/<pid>/fd/<fd>") which fails
# in Docker Desktop on macOS. Works on native Linux or Lima VM.
# See README.md "Known Issues" for workarounds.
RUN git clone https://github.com/sthima/libstapsdt.git /tmp/libstapsdt && \
    cd /tmp/libstapsdt && \
    make && \
    make install && \
    ldconfig && \
    rm -rf /tmp/libstapsdt

WORKDIR /app

RUN go mod init usdt

COPY app/libstabst/main.go ./
COPY entry.sh ./

RUN go mod tidy
RUN CGO_ENABLED=1 go build -o main .

ENTRYPOINT ["./entry.sh"]
