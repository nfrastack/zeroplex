BINARY_NAME := zt-dns-companion
BUILD_DIR := ./cmd/zt-dns-companion
GO := go
LDFLAGS := "-s -w"
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git describe --always --dirty || echo "dev")
BUILD_FLAGS := "-X main.Version=$(VERSION)"

all: build

build:
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME) $(BUILD_DIR)

build-release:
	$(GO) build -ldflags "$(LDFLAGS) $(BUILD_FLAGS)" -o $(BINARY_NAME) $(BUILD_DIR)

build-all:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS) $(BUILD_FLAGS)" -o $(BINARY_NAME)_x86_64 $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS) $(BUILD_FLAGS)" -o $(BINARY_NAME)_aarch64 $(BUILD_DIR)

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-x86_64 $(BINARY_NAME)-aarch64

install:
	$(GO) install $(BUILD_DIR)

release: clean build-all
	@echo "Binaries built with version: $(VERSION) and ready for release: $(BINARY_NAME)_x86_64, $(BINARY_NAME)_aarch64"

check-release:
	@if git describe --tags --exact-match >/dev/null 2>&1; then \
		if git diff-index --quiet HEAD --; then \
			echo "Repository is clean and tagged. Ready for release."; \
		else \
			echo "Repository is tagged but has uncommitted changes. Please commit or stash changes before release."; \
			exit 1; \
		fi; \
	else \
		echo "Repository is not tagged. Please create a tag before release."; \
		exit 1; \
	fi

help:
	@echo "make build           Build the binary"
	@echo "make build-release   Build the binary with version information"
	@echo "make build-all       Build binaries for x86_64 and aarch64"
	@echo "make clean           Clean up build artifacts"
	@echo "make install         Install the binary locally"
	@echo "make release         Build and prepare for release"
	@echo "make check-release   Verify if the repository is tagged and clean"
	@echo "make help            Show this message"
