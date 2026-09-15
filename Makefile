SHELL := /bin/bash

BIN     := ackctl
CMD     := ./cmd/ackctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go build -ldflags '$(LDFLAGS)' -o ./bin/$(BIN) $(CMD)

install: build
	cp ./bin/$(BIN) $(shell go env GOPATH)/bin/$(BIN)

# Everything that needs neither credentials nor a cluster.
test: lint unit-test unit-test-race

lint:
	go build ./...
	go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

unit-test:
	go test ./...

unit-test-race:
	go test -race ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf ./bin

.PHONY: build install test lint unit-test unit-test-race fmt clean
