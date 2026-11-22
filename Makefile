# Rackmon bauen. `make` baut für diesen Rechner, `make dist` für alle Zielsysteme.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
TARGETS := linux/amd64 linux/arm64 linux/arm darwin/arm64 windows/amd64

.PHONY: all test dist
all:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/rackmon ./cmd/rackmon

test:
	go vet ./...
	go test -race ./...

dist:
	@mkdir -p dist
	@for t in $(TARGETS); do \
	  os=$${t%/*}; arch=$${t#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
	  out=dist/rackmon-$$os-$$arch$$ext; \
	  echo "baue $$out"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch GOARM=7 go build -trimpath -ldflags "$(LDFLAGS)" -o $$out ./cmd/rackmon || exit 1; \
	done
	@cd dist && shasum -a 256 rackmon-* > SHA256SUMS
