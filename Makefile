# Where `make install` puts the binary. ~/bin is on PATH, so `site-audit`
# runs from any folder. Override with: make install PREFIX=/usr/local/bin
PREFIX  ?= $(HOME)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build install uninstall test

build:
	go build -ldflags "$(LDFLAGS)" -o site-audit .

install: build
	mkdir -p $(PREFIX)
	install -m 0755 site-audit $(PREFIX)/site-audit
	@echo "Installed $(PREFIX)/site-audit ($(VERSION))"

uninstall:
	rm -f $(PREFIX)/site-audit

test:
	go test ./...
