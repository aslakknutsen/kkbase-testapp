# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

USER 0

WORKDIR /build

# Install build dependencies
RUN dnf install -y --setopt=tsflags=nodocs \
    git \
    make && \
    dnf clean all

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build testservice
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o testservice ./cmd/testservice

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install CA certificates
RUN microdnf install -y ca-certificates && \
    microdnf clean all

WORKDIR /app

# Copy binary
COPY --from=builder /build/testservice .

# Expose ports
EXPOSE 8080 9090 9091

# Run
ENTRYPOINT ["/app/testservice"]

