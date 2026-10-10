BINARY := godot-cli
CMD := ./cmd/godot-cli
BIN_DIR := ./bin
INSTALL_DIR := $(HOME)/.local/bin

TEST_PACKAGES ?= ./tests/core
TEST_ARGS ?=

# WARNING: Benchmarks invoke a model and may cost money
# BENCHMARK_APPROACH: godot-cli | direct-files | both
# BENCHMARK_RUNS > 1 or BENCHMARK_APPROACH=both prints a median summary.

BENCHMARK_MODEL ?= opencode-go/deepseek-v4.1-flash
BENCHMARK_APPROACH ?= godot-cli
BENCHMARK_TASK ?= move_player
BENCHMARK_RUNS ?= 1
BENCHMARK_ARGS ?=

ifeq ($(OS),Windows_NT)
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -NonInteractive -Command
BINARY := godot-cli.exe
INSTALL_DIR := $(USERPROFILE)/.local/bin
endif

export GOBIN = $(INSTALL_DIR)

.PHONY: build install run format format-check vet test check clean benchmark benchmark-smoke

build:
ifeq ($(OS),Windows_NT)
	New-Item -ItemType Directory -Force -Path '$(BIN_DIR)' | Out-Null
else
	mkdir -p $(BIN_DIR)
endif
	go build -o "$(BIN_DIR)/$(BINARY)" $(CMD)

install:
	go install $(CMD)

run:
	go run $(CMD)

format:
	go fmt ./...

format-check:
ifeq ($(OS),Windows_NT)
	@$$files = gofmt -l .; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; if ($$files) { Write-Output 'Files need formatting:'; $$files; exit 1 }
else
	@test -z "$$(gofmt -l .)" || \
		(echo "Files need formatting:" && gofmt -l . && exit 1)
endif

vet:
	go vet ./...

test:
	go test -count=1 $(TEST_ARGS) $(TEST_PACKAGES)

check: format-check vet

benchmark:
	go run ./benchmarks \
		--model "$(BENCHMARK_MODEL)" \
		--approach "$(BENCHMARK_APPROACH)" \
		--task "$(BENCHMARK_TASK)" \
		--runs "$(BENCHMARK_RUNS)" \
		$(BENCHMARK_ARGS)

benchmark-smoke:
	go run ./benchmarks --smoke-api

clean:
ifeq ($(OS),Windows_NT)
	@$$target = [IO.Path]::GetFullPath('$(BIN_DIR)'); $$root = [IO.Path]::GetFullPath('.').TrimEnd('\') + '\'; if (-not $$target.StartsWith($$root, [StringComparison]::OrdinalIgnoreCase)) { throw 'BIN_DIR must be inside the workspace' }; if (Test-Path -LiteralPath $$target) { Remove-Item -LiteralPath $$target -Recurse -Force -ErrorAction Stop }
else
	rm -rf $(BIN_DIR)
endif
