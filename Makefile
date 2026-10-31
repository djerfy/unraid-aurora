SHELL := /bin/bash
NPM   := npm
GO    := go
BIN   := bin/aurora
TMP_BIN ?= /tmp/aurora-linux-amd64

.PHONY: all install build ui backend run dev-api dev-ui test vet fmt clean build-tmp

all: build

install:
	cd frontend && $(NPM) install --no-audit --no-fund

build: ui backend

ui:
	cd frontend && $(NPM) run build
	rm -rf backend/internal/webui/dist
	cp -R frontend/dist backend/internal/webui/dist

backend:
	cd backend && $(GO) build -trimpath -ldflags "-s -w" -o ../$(BIN) ./cmd/aurorad

run: build
	./$(BIN)

build-tmp:
	AURORA_TMP_BIN=$(TMP_BIN) ./scripts/build-tmp.sh

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

clean:
	rm -rf bin frontend/dist
	rm -rf backend/internal/webui/dist
	mkdir -p backend/internal/webui/dist
	printf '<!doctype html>\n<html><body>UI not built yet. Run <code>make build</code>.</body></html>\n' > backend/internal/webui/dist/index.html
