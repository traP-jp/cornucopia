# Stage 1
FROM golang:1.25-trixie AS proto-builder

WORKDIR /temp

RUN apt-get update && \
    apt-get install -y unzip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v33.2/protoc-33.2-linux-x86_64.zip -O protobuf.zip && \
    unzip -o protobuf.zip -d protobuf && \
    chmod -R 755 protobuf/*

ENV PATH $PATH:/temp/protobuf/bin


RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Stage 2
FROM alpine:latest AS downloader

RUN apk add --no-cache wget

WORKDIR /proto
RUN wget https://raw.githubusercontent.com/traP-jp/plutus/main/specs/protobuf/cornucopia.proto -O cornucopia.proto

# Stage 3
FROM proto-builder AS generator

ARG UID
ARG GID
ENV UID=${UID:-1000}
ENV GID=${GID:-1000}

COPY --from=downloader /proto /proto

WORKDIR /dist

RUN protoc \
        --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        -I/proto \
        cornucopia.proto

RUN go mod init github.com/traP-jp/plutus/api/protobuf
RUN go mod tidy
RUN chown "${UID}:${GID}" -R /dist

FROM golang:1.25.5-alpine AS go-builder

WORKDIR /app

RUN apk add --no-cache git

COPY --from=generator /dist ./api
COPY go.mod go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /cornucopia ./cmd/cornucopia

# Stage 4
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=go-builder /cornucopia .

EXPOSE 50051

COPY --from=generator /dist /dist
ENTRYPOINT cp /dist/* /api/protobuf
# CMD ["./cornucopia"]
