.PHONY: all build proto clean test docker-build help

# Variables
TESTSERVICE_BINARY=testservice
TESTGEN_BINARY=testgen
DOCKER_IMAGE ?= testservice:latest
PROTO_DIR=proto/testservice
PROTO_FILE=$(PROTO_DIR)/service.proto

all: proto build

help:
	@echo "TestApp Makefile"
	@echo "================"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build testservice and testgen binaries"
	@echo "  proto         - Generate Go code from protobuf"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run tests"
	@echo "  docker-build  - Build Docker image"
	@echo "  examples      - Generate all example manifests"
	@echo "  help          - Show this help message"

proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

build: proto
	@echo "Building testservice..."
	go build -o $(TESTSERVICE_BINARY) ./cmd/testservice
	@echo "Building testgen..."
	go build -o $(TESTGEN_BINARY) ./cmd/testgen
	@echo "Build complete!"

clean:
	@echo "Cleaning..."
	rm -f $(TESTSERVICE_BINARY) $(TESTGEN_BINARY)
	rm -f $(PROTO_DIR)/*.pb.go
	rm -rf output/
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	go test ./...

docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
	@echo "Docker image built: $(DOCKER_IMAGE)"

examples: build
	@echo "Generating example manifests..."
	@mkdir -p output
	./$(TESTGEN_BINARY) generate examples/simple-web/app.yaml -o output
	./$(TESTGEN_BINARY) generate examples/ecommerce/app.yaml -o output
	./$(TESTGEN_BINARY) generate examples/microservices/app.yaml -o output
	@echo "Examples generated in output/"

tidy:
	@echo "Tidying go modules..."
	go mod tidy

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint:
	@echo "Running linter..."
	golangci-lint run || true

install:
	@echo "Installing binaries to GOPATH/bin..."
	go install ./cmd/testservice
	go install ./cmd/testgen

