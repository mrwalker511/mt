.PHONY: all build test vet check clean apple-bridge install install-apple-bridge

all: vet test build

# Version stamped into the binary (mt --version): the nearest git tag, or the commit.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o mt .

test:
	go test ./...

vet:
	go vet ./...

# Same checks as CI: formatting, vet, staticcheck, tests with the race detector.
check:
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
	go test -race ./...

clean:
	rm -f mt mt-apple-bridge

apple-bridge:
	swiftc cmd/apple-llm/main.swift -o mt-apple-bridge
	codesign --entitlements cmd/apple-llm/apple-llm.entitlements -s - mt-apple-bridge
	@echo "Built ./mt-apple-bridge; place it alongside ./mt or in your PATH"

install: build
	install -m 0755 mt /usr/local/bin/mt
	@echo "Installed mt → /usr/local/bin/mt"

install-apple-bridge: apple-bridge
	install -m 0755 mt-apple-bridge /usr/local/bin/mt-apple-bridge
	@echo "Installed mt-apple-bridge → /usr/local/bin/mt-apple-bridge"
	@echo "Clear Gatekeeper quarantine if needed: xattr -d com.apple.quarantine /usr/local/bin/mt-apple-bridge"
