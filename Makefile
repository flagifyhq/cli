APP_NAME = flagify
BUILD_DIR = bin
MAIN = ./cmd/flagify

VERSION ?= dev
COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS = -ldflags "-X github.com/flagifyhq/cli/cmd.Version=$(VERSION) -X github.com/flagifyhq/cli/cmd.Commit=$(COMMIT)"

.PHONY: build run install clean test lint

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN)

run:
	go run $(MAIN) $(ARGS)

install:
	go install -v $(LDFLAGS) $(MAIN)

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

lint:
	go vet ./...
