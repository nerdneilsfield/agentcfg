SHELL := /bin/sh

PKG := agentcfg
GOFILES := cmd internal

BINARY := agentcfg
GORELEASER ?= goreleaser

.PHONY: all build fmt fmt-check lint vet test bench check clean release-check release-snapshot

all: build

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/agentcfg

fmt:
	gofumpt -l -w $(GOFILES)
	goimports -w -local $(PKG) $(GOFILES)
	gofmt -w $(GOFILES)

fmt-check:
	@test -z "$$(gofumpt -l $(GOFILES))" || { echo "gofumpt: files need formatting"; exit 1; }
	@test -z "$$(goimports -l -local $(PKG) $(GOFILES))" || { echo "goimports: files need formatting"; exit 1; }
	@test -z "$$(gofmt -l $(GOFILES))" || { echo "gofmt: files need formatting"; exit 1; }

lint:
	golangci-lint run ./...

vet:
	go vet ./...

test:
	CGO_ENABLED=0 go test ./...

bench:
	CGO_ENABLED=0 go test -bench=. -run=NONE ./internal/ir

check: fmt-check lint vet test release-check

clean:
	rm -f $(BINARY)
	rm -rf dist

release-check:
	$(GORELEASER) check

release-snapshot:
	$(GORELEASER) release --snapshot --clean
