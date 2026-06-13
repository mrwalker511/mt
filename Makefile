.PHONY: all build test vet clean apple-bridge

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
	swiftc cmd/apple-llm/main.swift -o mt-apple-bridge
	codesign --entitlements cmd/apple-llm/apple-llm.entitlements -s - mt-apple-bridge
	@echo "Built ./mt-apple-bridge — place it alongside ./mt or in your PATH"
