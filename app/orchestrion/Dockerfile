ARG runtime_version
FROM golang:${runtime_version}-bookworm

WORKDIR /app

RUN go mod init orchestrion

COPY app/main.go ./
COPY app/orchestrion/binaries/ /binaries/
COPY app/orchestrion/orchestrion.yml ./
COPY orchestrion.tool.go ./
COPY entry.sh ./

RUN go install github.com/DataDog/orchestrion@latest

RUN go mod tidy
RUN GOFLAGS="-mod=mod" orchestrion go build -o main .

ENTRYPOINT ["./entry.sh"]