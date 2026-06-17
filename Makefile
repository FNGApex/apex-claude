BINARY := bin/apex
PKG    := ./cmd/apex

.PHONY: build test fmt vet clean release install

build:
	go build -trimpath -ldflags "-s -w" -o $(BINARY) $(PKG)

# Build + register + enable the plugin and install the Apex output style.
install:
	bash scripts/install.sh

test:
	go test ./...

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

# Cross-compile a release matrix into bin/<os>-<arch>/apex
RELEASE_TARGETS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64
release:
	@for t in $(RELEASE_TARGETS); do \
	  os=$${t%/*}; arch=$${t#*/}; ext=""; \
	  [ "$$os" = "windows" ] && ext=".exe"; \
	  echo "building $$os/$$arch"; \
	  GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o bin/$$os-$$arch/apex$$ext $(PKG); \
	done
