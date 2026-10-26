VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test demo pi check

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o vahti ./cmd/vahti

# Raspberry Pi 3/4/5 with a 64-bit OS
pi:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o vahti ./cmd/vahti

test:
	go test -race -count=1 ./...

check: test
	go vet ./...
	test -z "$$(gofmt -l .)"

demo: build
	./vahti demo
