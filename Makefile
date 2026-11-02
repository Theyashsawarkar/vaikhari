# Build:                             make
# System-wide or packaged install:   make install [PREFIX=/usr/local] [DESTDIR=]
# Per-user, without make:            ./install.sh
PREFIX ?= /usr/local
BINDIR  = $(DESTDIR)$(PREFIX)/bin
SHARE   = $(DESTDIR)$(PREFIX)/share

.PHONY: build install uninstall check test lint dist

build:
	go build -trimpath -o bin/vaikhari ./cmd/vaikhari

install: build
	install -Dm755 bin/vaikhari $(BINDIR)/vaikhari
	install -Dm644 README.md $(SHARE)/doc/vaikhari/README.md
	install -Dm644 CHANGELOG.md $(SHARE)/doc/vaikhari/CHANGELOG.md
	install -Dm644 LICENSE $(SHARE)/licenses/vaikhari/LICENSE

uninstall:
	rm -f $(BINDIR)/vaikhari
	rm -rf $(SHARE)/doc/vaikhari $(SHARE)/licenses/vaikhari

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	shellcheck -S warning vaikhari.tmux install.sh scripts/vaikhari-run

# What CI runs.
check: lint test build
	./bin/vaikhari --version
	./bin/vaikhari --help >/dev/null

# Release binaries (CI does this on a tag).
dist:
	mkdir -p dist
	for arch in amd64 arm64; do \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags='-s -w' -o dist/vaikhari-linux-$$arch ./cmd/vaikhari; \
	done
