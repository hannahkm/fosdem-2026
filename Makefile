.PHONY: all build pdf html watch lint fix format clean install help

.PHONY: format/md lint/md fix/md
.PHONY: install/typos check/typos fix/typos install/ratchet ratchet/pin ratchet/upgrade ratchet/lint
.PHONY: deps deps/go build/go run/go test/go lint/go fix/go format/go clean/go check/go all/go install/golangci-lint

all: build format lint

MARP_CONFIG := marp/marp.config.js
SRC := presentation.md
COMMON_FLAGS := --allow-local-files --theme-set marp/css/

# OS detection for tool installation
UNAME_S := $(shell uname -s)
TYPOS := $(shell command -v typos 2> /dev/null)
RATCHET := $(shell command -v ratchet 2> /dev/null)
GOLANGCI_LINT := $(shell command -v golangci-lint 2> /dev/null)

build: pdf build/go
format: format/md format/go
lint: lint/md lint/go
fix: fix/md fix/go

pdf:
	npx marp --config $(MARP_CONFIG) $(SRC) $(COMMON_FLAGS) -o presentation.pdf

html:
	npx marp --config $(MARP_CONFIG) $(SRC) $(COMMON_FLAGS) -o presentation.html

watch:
	npx marp --config $(MARP_CONFIG) $(SRC) $(COMMON_FLAGS) --watch --preview

lint/md:
	npx markdownlint-cli2 "**/*.md" "#node_modules" "#experiments"

fix/md:
	npx markdownlint-cli2 --fix "**/*.md" "#node_modules" "#experiments"

format/md:
	npx prettier --write "*.md"

install/typos:
ifndef TYPOS
ifeq ($(UNAME_S),Darwin)
	brew install typos-cli
else ifeq ($(UNAME_S),Linux)
	cargo install typos-cli
else
	@echo "Please install typos-cli manually: https://github.com/crate-ci/typos"
	@exit 1
endif
endif

check/typos: install/typos
	typos

fix/typos: install/typos
	typos --write-changes

install/ratchet:
ifndef RATCHET
ifeq ($(UNAME_S),Darwin)
	brew install ratchet
else
	go install github.com/sethvargo/ratchet@latest
endif
endif

ratchet/pin: install/ratchet
	ratchet pin .github/workflows/*.yml

ratchet/upgrade: install/ratchet
	ratchet upgrade .github/workflows/*.yml

ratchet/lint: install/ratchet
	ratchet lint .github/workflows/*.yml

# Go tooling
install/golangci-lint:
ifndef GOLANGCI_LINT
ifeq ($(UNAME_S),Darwin)
	brew install golangci-lint
else
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
endif
endif

deps/go:
	go mod tidy

deps: deps/go install

build/go:
	go build -o fosdem .

run/go:
	go run . run --scenario default --num 1

test/go:
	go test -v -race ./...

lint/go: install/golangci-lint
	golangci-lint run

fix/go: install/golangci-lint
	golangci-lint run --fix

format/go:
	golangci-lint fmt

clean/go:
	rm -f fosdem

check/go: lint/go test/go

all/go: deps/go format/go lint/go build/go

clean:
	rm -f presentation.pdf presentation.html

install:
	npm install

help:
	@echo "Combined:"
	@echo "  make all          - Build, format, and lint everything"
	@echo "  make format       - Format all code (markdown + Go)"
	@echo "  make lint         - Lint all code (markdown + Go)"
	@echo "  make fix          - Auto-fix all issues (markdown + Go)"
	@echo ""
	@echo "Presentation:"
	@echo "  make build        - Generate PDF"
	@echo "  make html         - Generate HTML"
	@echo "  make watch        - Live preview"
	@echo "  make lint/md      - Lint markdown"
	@echo "  make fix/md       - Auto-fix markdown issues"
	@echo "  make format/md    - Format markdown"
	@echo "  make check/typos  - Check for typos"
	@echo "  make fix/typos    - Fix typos automatically"
	@echo ""
	@echo "Go:"
	@echo "  make build/go     - Build CLI binary"
	@echo "  make run/go       - Run default scenario"
	@echo "  make test/go      - Run tests with race detector"
	@echo "  make lint/go      - Lint Go code"
	@echo "  make fix/go       - Auto-fix Go linting issues"
	@echo "  make format/go    - Format Go code"
	@echo "  make check/go     - Run lint and tests"
	@echo "  make all/go       - Full Go build pipeline"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps         - Install all dependencies"
	@echo "  make deps/go      - Tidy Go modules"
	@echo "  make install      - Install npm dependencies"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean        - Remove presentation files"
	@echo "  make clean/go     - Remove Go binary"
