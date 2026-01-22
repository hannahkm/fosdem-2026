ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init obi

COPY app/main.go ./
COPY app/obi/binaries/ /binaries/
COPY entry.sh ./

RUN go mod tidy
RUN go build -o main .

ENTRYPOINT ["./entry.sh"]