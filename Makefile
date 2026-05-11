BINARY     := podfails
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: all build clean install uninstall tidy fmt vet

all: build

## build: compile the binary into the project root
build:
	go build -o $(BINARY) ./cmd/podfails

## install: build and install the binary to $(INSTALL_DIR)
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"

## uninstall: remove the installed binary
uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

## clean: remove the compiled binary
clean:
	rm -f $(BINARY)

## tidy: tidy Go module dependencies
tidy:
	go mod tidy

## fmt: format all Go source files
fmt:
	go fmt ./...

## vet: run go vet
vet:
	go vet ./...

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
