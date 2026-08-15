APP_NAME := movieswipe
BIN_DIR  := dist

.PHONY: all build build-linux-amd64 build-linux-arm64 test fmt vet clean run

all: fmt vet test build

build:
	go build -o $(BIN_DIR)/$(APP_NAME) .

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 .

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/$(APP_NAME)-linux-arm64 .

clean:
	rm -rf $(BIN_DIR)

test:
	go test -cover ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

run:
	go run .
