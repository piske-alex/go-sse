# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOLINT=golangci-lint
BINARY_NAME=go-sse
BINARY_DIR=bin

.PHONY: all build clean run test lint

all: test build

build:
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/server

clean:
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	go clean -testcache

run: build
	./$(BINARY_DIR)/$(BINARY_NAME)

test:
	$(GOTEST) -v ./...

lint:
	$(GOLINT) run

deps:
	$(GOGET) -u
	dlv debug ./cmd/server/ -- serve

.DEFAULT_GOAL := build
