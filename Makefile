BINARY  := tocommit
PKG     := github.com/K1NGS1LVER/ToGitOrNotToGit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/cmd.version=$(VERSION)
GOFLAGS := -trimpath

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: build test vet fmt lint tidy cross release clean

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/tocommit

test:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run

tidy:
	go mod tidy

cross:
	@rm -rf dist && mkdir -p dist
	@for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; goarch=$${platform#*/}; ext=""; \
		if [ "$$goos" = "windows" ]; then ext=".exe"; fi; \
		echo "building dist/$(BINARY)-$$goos-$$goarch$$ext"; \
		GOOS=$$goos GOARCH=$$goarch go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
			-o "dist/$(BINARY)-$$goos-$$goarch$$ext" ./cmd/tocommit || exit 1; \
	done

release:
	goreleaser release --clean

clean:
	rm -rf dist $(BINARY)
