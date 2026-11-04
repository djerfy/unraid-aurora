SHELL := /bin/bash
NPM   := npm
GO    := go
BIN   := bin/aurora
TMP_BIN ?= /tmp/aurora-linux-amd64

# Tags use vX.Y.Z; the binary and the UI display X.Y.Z. Override with
# `make VERSION=...` for a one-off build.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
ifeq ($(strip $(VERSION)),)
VERSION := 0.0.0-dev
endif
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all install build ui backend run dev-api dev-ui test vet fmt clean build-tmp openapi

all: build

install:
	cd frontend && $(NPM) install --no-audit --no-fund

build: ui backend

ui:
	cd frontend && $(NPM) run build
	rm -rf backend/internal/webui/dist
	cp -R frontend/dist backend/internal/webui/dist

backend:
	cd backend && $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/aurorad

run: build
	./$(BIN)

build-tmp:
	@set -e; \
	root="$(CURDIR)"; \
	restore=$$(mktemp -d); \
	cleanup() { cd "$$root"; rm -rf backend/internal/webui/dist; if [ -d "$$restore/dist" ]; then cp -R "$$restore/dist" backend/internal/webui/dist; fi; rm -rf "$$restore"; }; \
	trap cleanup EXIT; \
	if [ -d backend/internal/webui/dist ]; then cp -R backend/internal/webui/dist "$$restore/dist"; fi; \
	$(MAKE) --no-print-directory ui; \
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(TMP_BIN) ./cmd/aurorad; \
	cd "$$root"; \
	echo; \
	echo "binary: $(TMP_BIN)"; \
	file $(TMP_BIN); \
	ls -lh $(TMP_BIN); \
	shasum -a 256 $(TMP_BIN) 2>/dev/null || sha256sum $(TMP_BIN)

dev-api:
	cd backend && $(GO) run ./cmd/aurorad

dev-ui:
	cd frontend && $(NPM) run dev

test:
	cd backend && $(GO) test ./...

vet:
	cd backend && $(GO) vet ./...

fmt:
	cd backend && gofmt -w .

openapi:
	python3 scripts/openapi.py

clean:
	rm -rf bin frontend/dist
	rm -rf backend/internal/webui/dist
	mkdir -p backend/internal/webui/dist
	printf '<!doctype html>\n<html><body>UI not built yet. Run <code>make build</code>.</body></html>\n' > backend/internal/webui/dist/index.html
