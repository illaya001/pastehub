BINARY := pastehub
BIN_DIR := bin
PKG := ./cmd/pastehub
GO ?= go
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build build-current build-platforms test clean run fmt vet

all: build

build: build-platforms

build-current:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(BINARY) $(PKG)

build-platforms:
	mkdir -p $(BIN_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BIN_DIR)/$(BINARY)-$$os-$$arch; \
		echo "Building $$output"; \
		GOOS=$$os GOARCH=$$arch $(GO) build -o $$output $(PKG); \
	done

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

run:
	$(GO) run $(PKG) serve

clean:
	rm -rf $(BIN_DIR)
