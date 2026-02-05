# Varnish - Environment Variable Manager
# Run 'make' or 'make help' to see available targets

BINARY := varnish
VERSION := $(shell cat version)
LDFLAGS := -ldflags="-w -s -X github.com/dk/varnish/internal/cli.Version=$(VERSION)"

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: config
config: ## Create .env from example.env (interpolates env vars)
	@if [ ! -f .env ]; then \
		if command -v envsubst >/dev/null 2>&1; then \
			envsubst < example.env > .env && echo "Created .env"; \
		else \
			while IFS= read -r line || [ -n "$$line" ]; do \
				case "$$line" in \
					''|'#'*) printf '%s\n' "$$line";; \
					*) eval printf '%s\n' "\"$$line\"";; \
				esac; \
			done < example.env > .env && echo "Created .env"; \
		fi \
	else \
		echo ".env already exists"; \
	fi

.PHONY: build
build: ## Build the binary
	go build $(LDFLAGS) -o $(BINARY) ./cmd/varnish

.PHONY: run
run: build ## Build and run
	./$(BINARY)

.PHONY: test
test: ## Run tests
	go test -race ./...

.PHONY: lint
lint: ## Run linter (installs golangci-lint if missing)
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@PATH="$$PATH:$$(go env GOPATH)/bin" golangci-lint run

.PHONY: fmt
fmt: ## Format code
	gofmt -w .

.PHONY: coverage
coverage: ## Run tests with coverage
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out coverage.html
	rm -rf dist/

# Release targets for cross-compilation
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: release
release: clean ## Build release binaries for all platforms
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		go build $(LDFLAGS) -o dist/$(BINARY)-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe") ./cmd/varnish && \
		echo "Built: $(BINARY)-$${platform%/*}-$${platform#*/}"; \
	done
	@echo "Release binaries in dist/"

.PHONY: release-checksums
release-checksums: release ## Generate checksums for release binaries
	@cd dist && sha256sum * > checksums.txt
	@echo "Checksums written to dist/checksums.txt"

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(BINARY):$(VERSION) -t $(BINARY):latest --build-arg VERSION=$(VERSION) .

.PHONY: version
version: ## Show version
	@echo $(VERSION)

.PHONY: install
install: build ## Install to GOPATH/bin
	cp $(BINARY) $(GOPATH)/bin/

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	go mod tidy

.DEFAULT_GOAL := help
