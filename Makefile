BINARY_NAME := zt-dns-companion
BUILD_DIR := ./cmd/zt-dns-companion
GO := go
LDFLAGS := "-s -w"
VERSION := $(shell git describe --tags --always --dirty)
BUILD_FLAGS := "-X main.Version=$(VERSION)"

all: build

build:
	$(GO) build -o $(BINARY_NAME) $(BUILD_DIR)

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

help:
	@echo "make build           Build the binary"
	@echo "make build-release   Build the binary with version information"
	@echo "make build-all       Build binaries for x86_64 and aarch64"
	@echo "make clean           Clean up build artifacts"
	@echo "make install         Install the binary locally"
	@echo "make release         Build and prepare for release"
	@echo "make help            Show this help message"