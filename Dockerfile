FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Copy local module that is referenced via replace directive
COPY api/ api/

RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /cornucopia ./cmd/cornucopia

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /cornucopia .

EXPOSE 50051

CMD ["./cornucopia"]
