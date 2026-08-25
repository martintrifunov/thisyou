BREW_PREFIX := $(shell command -v brew >/dev/null 2>&1 && brew --prefix)
ifeq ($(BREW_PREFIX),)
BINDIR ?= /usr/local/bin
else
BINDIR ?= $(BREW_PREFIX)/bin
endif

.PHONY: build install run test vet clean

build:
	go build -trimpath -o thisyou ./cmd/thisyou

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 thisyou "$(BINDIR)/thisyou"
	@echo "installed $(BINDIR)/thisyou"

run:
	go run ./cmd/thisyou

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f thisyou
