GO ?= go
NPM ?= npm

.PHONY: help setup install dev backend frontend test lint typecheck build embed release docker-build docker-validate clean check

help:
	@echo "DataDeck development commands:"
	@echo "  make setup      Install frontend deps and download Go modules"
	@echo "  make dev        Start the development environment (frontend)"
	@echo "  make backend    Run the Go backend"
	@echo "  make frontend   Run the Next.js dev server"
	@echo "  make test       Run backend and frontend tests"
	@echo "  make lint       Run backend go vet and frontend eslint"
	@echo "  make typecheck  Compile backend and run frontend tsc --noEmit"
	@echo "  make build      Build backend binary and frontend production bundle"
	@echo "  make clean      Remove build artifacts"
	@echo "  make check      Run lint + typecheck"

setup:
	cd backend && $(GO) mod download
	cd frontend && $(NPM) install

install: setup

dev:
	@echo "DataDeck dev environment"
	@echo "  frontend: http://localhost:3000"
	@echo "  backend : M0 bootstrap placeholder (no HTTP server yet); run 'make backend' separately"
	@$(MAKE) --no-print-directory frontend

backend:
	$(MAKE) -C backend run

frontend:
	cd frontend && $(NPM) run dev

test:
	$(MAKE) -C backend test
	cd frontend && $(NPM) run test

lint:
	$(MAKE) -C backend vet
	cd frontend && $(NPM) run lint

typecheck:
	$(MAKE) -C backend build
	cd frontend && $(NPM) run typecheck

# Canonical single-binary build: installs frontend deps, produces the static
# export and embeds it into a CGO-free Go executable at backend/bin/datadeck.
build:
	bash scripts/build-embedded.sh

# Alias kept for clarity in release docs.
embed: build

# Cross-platform release binaries (see docs/release/platform-matrix.md).
release:
	bash scripts/build-release.sh

# Container image (see docs/release/docker.md).
docker-build:
	docker build -f docker/Dockerfile --build-arg VERSION=$$(cat VERSION) -t datadeck:$$(cat VERSION) .

docker-validate:
	bash scripts/docker-validate.sh

clean:
	$(MAKE) -C backend clean
	cd frontend && $(NPM) run clean
	rm -rf release

check: lint typecheck
