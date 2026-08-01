.PHONY: build install clean test lint fmt

BINARY  := clavis
# Derive the version from git (tag or short SHA); override with `make VERSION=…`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/clavis/

install: build
	mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/$(BINARY)
	@echo "Installed $(VERSION) to ~/.local/bin/$(BINARY)"

clean:
	rm -rf bin/ dist/

test:
	go test -race ./...

lint:
	go vet ./...

fmt:
	gofmt -w .
