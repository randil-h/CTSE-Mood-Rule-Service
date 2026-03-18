.PHONY: proto build test docker-build docker-up docker-down clean lint

# Generate protobuf files
proto:
	@echo "Generating protobuf files..."
	@mkdir -p proto/moodrule proto/auth proto/productcatalog
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/mood_rule_service.proto
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth_service.proto
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/product_catalog.proto

# Build the application
build: proto
	@echo "Building application..."
	@go build -o bin/mood-rule-service cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./internal/engine/...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t mood-rule-service:latest .

# Start all services with Docker Compose
docker-up:
	@echo "Starting services..."
	@docker-compose up -d

# Stop all services
docker-down:
	@echo "Stopping services..."
	@docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/ proto/**/*.pb.go coverage.out

# Lint code
lint:
	@echo "Linting..."
	@golangci-lint run

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Run the service locally
run: build
	@echo "Running service..."
	@./bin/mood-rule-service
