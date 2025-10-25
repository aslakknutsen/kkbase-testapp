# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install protoc and dependencies
RUN apk add --no-cache git protobuf-dev make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build testservice
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o testservice ./cmd/testservice

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /build/testservice .

# Expose ports
EXPOSE 8080 9090 9091

# Run
ENTRYPOINT ["/app/testservice"]

