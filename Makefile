BINARY   := ssmhop
PKG      := github.com/tryoverflow/ssmhop
GOLANGCI := v2.13.2
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/internal/cmd.Version=$(VERSION)

.DEFAULT_GOAL := all

.PHONY: all
all: tidy fmt vet test build

.PHONY: build
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" .

.PHONY: test
test:
	go test ./...

.PHONY: cover
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: fmt
fmt:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI) run

.PHONY: snapshot
snapshot:
	goreleaser release --snapshot --clean

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf bin coverage.out
