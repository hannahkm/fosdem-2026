ARG runtime_version
FROM golang:${runtime_version}-bookworm
WORKDIR /app
RUN go mod init injector
COPY app/injector/main.go ./
COPY app/injector/binaries/ /binaries/
COPY entry.sh ./
RUN go mod tidy
# Disable optimizations and inlining for cleaner function boundaries
# This ensures handler functions remain intact and hookable
RUN go build -gcflags="all=-N -l" -o main .
ENTRYPOINT ["./entry.sh"]
