.PHONY: all build test vet clean apple-bridge install install-apple-bridge

all: vet test build

build:
	go build -o mt .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f mt mt-apple-bridge

apple-bridge:
	swiftc -arch arm64 cmd/apple-llm/main.swift -o mt-apple-bridge
	codesign --entitlements cmd/apple-llm/apple-llm.entitlements -s - mt-apple-bridge
	@echo "Built ./mt-apple-bridge — place it alongside ./mt or in your PATH"

install: build
	install -m 0755 mt /usr/local/bin/mt
	@echo "Installed mt → /usr/local/bin/mt"

install-apple-bridge: apple-bridge
	install -m 0755 mt-apple-bridge /usr/local/bin/mt-apple-bridge
	@echo "Installed mt-apple-bridge → /usr/local/bin/mt-apple-bridge"
	@echo "Clear Gatekeeper quarantine if needed: xattr -d com.apple.quarantine /usr/local/bin/mt-apple-bridge"
