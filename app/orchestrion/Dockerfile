ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init orchestrion

COPY app/main.go ./
COPY app/orchestrion/binaries/ /binaries/
COPY app/orchestrion/orchestrion.yml ./

RUN go mod tidy
RUN orchestrion go build -o main .

ENTRYPOINT ["./main"]