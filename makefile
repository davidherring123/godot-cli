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

format:
	go fmt ./...

format-check:
	@test -z "$$(gofmt -l .)" || \
		(echo "Files need formatting:" && gofmt -l . && exit 1)

vet:
	go vet ./...

test:
	go test ./...

check: format-check vet test

clean:
	rm -rf $(BIN_DIR)