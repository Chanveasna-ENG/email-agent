.PHONY: all fmt vet test build clean

BINARY_NAME=email-agent.exe

all: fmt vet test build

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v -cover ./...

build:
	go build -v -o $(BINARY_NAME) .

clean:
	go clean
	powershell -Command "if (Test-Path $(BINARY_NAME)) { Remove-Item $(BINARY_NAME) }"
