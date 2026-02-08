VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
           -X github.com/reviewapps-dev/rad/internal/version.Version=$(VERSION) \
           -X github.com/reviewapps-dev/rad/internal/version.Commit=$(COMMIT) \
           -X github.com/reviewapps-dev/rad/internal/version.BuildDate=$(DATE)

.PHONY: build run-dev run-callback clean test release

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/rad ./cmd/rad
	CGO_ENABLED=0 go build -o bin/callback-server ./scripts/callback-server

run-dev: build
	./bin/rad --dev --token secret

run-callback:
	CGO_ENABLED=0 go build -o bin/callback-server ./scripts/callback-server
	./bin/callback-server

clean:
	rm -rf bin/ dist/

test:
	go test ./...

release:
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=0.1.0)
endif
	@echo "Building rad $(VERSION) for linux/amd64 and linux/arm64..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/rad_linux_amd64/rad ./cmd/rad
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/rad_linux_arm64/rad ./cmd/rad
	cd dist && tar -czf rad_linux_amd64.tar.gz rad_linux_amd64/
	cd dist && tar -czf rad_linux_arm64.tar.gz rad_linux_arm64/
	cd dist && sha256sum rad_linux_amd64.tar.gz rad_linux_arm64.tar.gz > checksums.txt
	@echo "Release artifacts in dist/"
