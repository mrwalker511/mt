.PHONY: all build test vet clean

all: vet test build

build:
	go build -o mt .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f mt
