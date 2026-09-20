BINARY := godot-cli
CMD := ./cmd/godot-cli
BIN_DIR := ./bin
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install run test clean fmt

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

install:
	GOBIN=$(HOME)/.local/bin go install $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

format:
	go fmt ./...

vet:
	go vet ./...

check: format vet test

clean:
	rm -rf $(BIN_DIR)