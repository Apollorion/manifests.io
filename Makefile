BIN := $(HOME)/bin/manifests
VERSION ?= $(shell git describe --always --dirty)

.PHONY: build install update-schemas frontend test check dev dev-api dev-web clean

build: frontend
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o build/manifests ./cmd/manifests
	node frontend/scripts/prerender.mjs

install: build
	install -d $(dir $(BIN))
	install -m 755 build/manifests $(BIN)

update-schemas:
	go run ./cmd/update-schemas $(UPDATE_ARGS)

frontend:
	npm --prefix frontend ci --no-audit --no-fund
	npm --prefix frontend run build

test:
	go test -race ./...
	npm --prefix frontend test -- --run

check: test
	go vet ./...
	npm --prefix frontend run typecheck
	tofu -chdir=infra fmt -check

dev:
	$(MAKE) -j2 dev-api dev-web

dev-api:
	go run ./cmd/manifests

dev-web:
	npm --prefix frontend run dev -- --host 127.0.0.1

clean:
	rm -rf build frontend/dist frontend/dist-server frontend/dist-render frontend/prerender frontend/prerender.tmp
