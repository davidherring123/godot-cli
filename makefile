BINARY := godot-mcp-host
CMD := ./cmd/godot-mcp-host
BIN_DIR := ./bin
DIST_DIR := ./dist
INSTALL_DIR := $(HOME)/.local/bin

VERSION ?= dev

TEST_PACKAGES ?= ./internal/... ./tests/...
TEST_ARGS ?=

# WARNING: Benchmarks invoke a model and may cost money
# BENCHMARK_APPROACH: mcp | direct-files | both
# both compares the MCP-first path against direct file editing.
# BENCHMARK_RUNS > 1 or BENCHMARK_APPROACH=both prints a median summary.

BENCHMARK_MODEL ?= deepseek/deepseek-flash
BENCHMARK_APPROACH ?= mcp
BENCHMARK_TASK ?= playable_platformer
BENCHMARK_RUNS ?= 1
BENCHMARK_ARGS ?=

ifeq ($(OS),Windows_NT)
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -NonInteractive -Command
BINARY := godot-mcp-host.exe
INSTALL_DIR := $(USERPROFILE)/.local/bin
endif

export GOBIN = $(INSTALL_DIR)

.PHONY: build install run format format-check vet lint test check clean release-addon benchmark benchmark-smoke

build:
ifeq ($(OS),Windows_NT)
	New-Item -ItemType Directory -Force -Path '$(BIN_DIR)' | Out-Null
else
	mkdir -p $(BIN_DIR)
endif
	go build -o "$(BIN_DIR)/$(BINARY)" $(CMD)

release-addon:
	go run ./cmd/package-addon --version "$(VERSION)" --out "$(DIST_DIR)"

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

lint:
	golangci-lint run ./...

test:
	go test -count=1 $(TEST_ARGS) $(TEST_PACKAGES)

check: format-check vet lint

benchmark:
	go run ./benchmarks \
		--model "$(BENCHMARK_MODEL)" \
		--approach "$(BENCHMARK_APPROACH)" \
		--task "$(BENCHMARK_TASK)" \
		--runs "$(BENCHMARK_RUNS)" \
		$(BENCHMARK_ARGS)

benchmark-smoke:
	go run ./benchmarks --smoke

clean:
ifeq ($(OS),Windows_NT)
	@$$root = [IO.Path]::GetFullPath('.').TrimEnd('\') + '\'; foreach ($$path in @('$(BIN_DIR)', '$(DIST_DIR)')) { $$target = [IO.Path]::GetFullPath($$path); if (-not $$target.StartsWith($$root, [StringComparison]::OrdinalIgnoreCase)) { throw "clean target must be inside the workspace: $$path" }; if (Test-Path -LiteralPath $$target) { Remove-Item -LiteralPath $$target -Recurse -Force -ErrorAction Stop } }
else
	rm -rf $(BIN_DIR) $(DIST_DIR)
endif
