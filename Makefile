.PHONY: build install clean test

BINARY  := clavis
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/clavis/

install: build
	mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/$(BINARY)
	@echo "Installed to ~/.local/bin/$(BINARY)"

clean:
	rm -rf bin/

test:
	go test ./...

lint:
	go vet ./...
