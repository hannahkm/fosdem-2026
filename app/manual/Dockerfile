ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init manual

COPY app/manual/main.go ./
COPY app/manual/binaries/ /binaries/
COPY entry.sh ./

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./entry.sh"]