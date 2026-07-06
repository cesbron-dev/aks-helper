BINARY  := aks-helper
PREFIX  ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/cesbron-dev/aks-helper/cmd.version=$(VERSION)

.PHONY: all build install install-skill test vet fmt lint check clean

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

# Install binary + agent skill globally (Claude Code personal skills dir).
install-skill:
	./scripts/install.sh

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed; see https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run

check: fmt vet test

clean:
	rm -rf bin
