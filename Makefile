.PHONY: all fmt vet test build build-linux docker-build docker-up docker-down clean

BINARY_NAME=email-agent.exe

all: fmt vet test build

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v -cover ./...

build:
	go build -v -o $(BINARY_NAME) ./cmd/email-agent

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -v -o bin/email-agent ./cmd/email-agent

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	go clean
	powershell -Command "if (Test-Path $(BINARY_NAME)) { Remove-Item $(BINARY_NAME) }"
	powershell -Command "if (Test-Path bin) { Remove-Item -Recurse -Force bin }"

