ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init default

COPY app/main.go ./
COPY app/default/binaries/ /binaries/

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./main"]