ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init ebpf

COPY app/main.go ./
COPY app/ebpf/binaries/ /binaries/
COPY entry.sh ./

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./entry.sh"]