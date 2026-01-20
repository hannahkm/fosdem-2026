ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init main

COPY app/main.go ./

RUN orchestrion go build main.go

ENTRYPOINT ["./main"]