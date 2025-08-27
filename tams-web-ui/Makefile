.PHONY: build run dev docker-build docker-run clean test

# Build the application
build:
	go build -o bin/tams-web-ui main.go

# Run the application locally
run: build
	./bin/tams-web-ui

# Run in development mode with hot reload
dev:
	go run main.go

# Build Docker image
docker-build:
	docker build -t tams-web-ui:latest .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker containers
docker-stop:
	docker-compose down

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/tams-web-ui-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/tams-web-ui-darwin-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o bin/tams-web-ui-windows-amd64.exe main.go