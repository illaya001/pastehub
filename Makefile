BINARY := pastehub
BIN_DIR := bin
PKG := ./cmd/pastehub
GO ?= go

.PHONY: all build test clean run fmt vet

all: build

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(BINARY) $(PKG)

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
