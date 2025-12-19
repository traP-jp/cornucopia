FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Install dependencies for protobuf generation
RUN apk add --no-cache git protobuf protobuf-dev

# Install Go protobuf plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Generate protobuf files dynamically
RUN go run ./cmd/genproto

# Download dependencies
RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /cornucopia ./cmd/cornucopia

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /cornucopia .

EXPOSE 50051

CMD ["./cornucopia"]
