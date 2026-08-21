PREFIX  ?= /usr
BINDIR   = $(PREFIX)/bin
POLKITDIR = $(PREFIX)/share/polkit-1/rules.d
SYSTEMDDIR = $(PREFIX)/lib/systemd/user
MANDIR    = $(PREFIX)/share/man/man1
COMPLETIONDIR_BASH = $(PREFIX)/share/bash-completion/completions
COMPLETIONDIR_ZSH  = $(PREFIX)/share/zsh/site-functions
COMPLETIONDIR_FISH = $(PREFIX)/share/fish/vendor_completions.d
LICENSEDIR = $(PREFIX)/share/licenses/rambo
DOCDIR     = $(PREFIX)/share/doc/rambo

BINARY   = rambo
GO       ?= go

# Version injected via ldflags. Overridden by PKGBUILD / GoReleaser / tagged builds.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X github.com/jashk120/rambo/internal/version.Version=$(VERSION) -X github.com/jashk120/rambo/internal/version.Commit=$(COMMIT) -X github.com/jashk120/rambo/internal/version.Date=$(DATE)
GOFLAGS ?= -trimpath -buildmode=pie -mod=readonly -modcacherw
BUILD   = $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)"

.PHONY: all build install uninstall clean man completions test vet fmt lint

all: build

build:
	$(BUILD) -o $(BINARY) .

man: build
	mkdir -p man
	./$(BINARY) completion bash >/dev/null 2>&1 || true
	# Generate man page via cobra if supported, otherwise use static docs/rambo.1
	@if [ -f docs/rambo.1 ]; then \
		mkdir -p man; cp docs/rambo.1 man/rambo.1; \
	fi

completions: build
	mkdir -p completions
	./$(BINARY) completion bash  > completions/rambo.bash
	./$(BINARY) completion zsh   > completions/_rambo
	./$(BINARY) completion fish  > completions/rambo.fish

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	install -Dm644 polkit/99-rambo.rules $(DESTDIR)$(POLKITDIR)/99-rambo.rules
	install -Dm644 systemd/rambo.service $(DESTDIR)$(SYSTEMDDIR)/rambo.service
	install -Dm644 LICENSE $(DESTDIR)$(LICENSEDIR)/LICENSE
	install -Dm644 README.md $(DESTDIR)$(DOCDIR)/README.md
	# man page
	@if [ -f docs/rambo.1 ]; then \
		install -Dm644 docs/rambo.1 $(DESTDIR)$(MANDIR)/rambo.1; \
	elif [ -f man/rambo.1 ]; then \
		install -Dm644 man/rambo.1 $(DESTDIR)$(MANDIR)/rambo.1; \
	fi
	# shell completions (best-effort: binary already built)
	@mkdir -p $(DESTDIR)$(COMPLETIONDIR_BASH) $(DESTDIR)$(COMPLETIONDIR_ZSH) $(DESTDIR)$(COMPLETIONDIR_FISH)
	@if ./$(BINARY) completion bash >/dev/null 2>&1; then \
		./$(BINARY) completion bash > $(DESTDIR)$(COMPLETIONDIR_BASH)/rambo; \
		./$(BINARY) completion zsh  > $(DESTDIR)$(COMPLETIONDIR_ZSH)/_rambo; \
		./$(BINARY) completion fish > $(DESTDIR)$(COMPLETIONDIR_FISH)/rambo.fish; \
	fi
	@if [ -z "$(DESTDIR)" ] && [ -n "$$XDG_RUNTIME_DIR" ] && command -v systemctl >/dev/null 2>&1; then \
		systemctl --user daemon-reload; \
	fi
	@echo "Installed. Enable with: systemctl --user enable --now rambo.service"

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -f $(DESTDIR)$(POLKITDIR)/99-rambo.rules
	rm -f $(DESTDIR)$(SYSTEMDDIR)/rambo.service
	rm -f $(DESTDIR)$(MANDIR)/rambo.1
	rm -f $(DESTDIR)$(LICENSEDIR)/LICENSE
	rm -f $(DESTDIR)$(COMPLETIONDIR_BASH)/rambo
	rm -f $(DESTDIR)$(COMPLETIONDIR_ZSH)/_rambo
	rm -f $(DESTDIR)$(COMPLETIONDIR_FISH)/rambo.fish
	@if [ -z "$(DESTDIR)" ] && [ -n "$$XDG_RUNTIME_DIR" ] && command -v systemctl >/dev/null 2>&1; then \
		systemctl --user daemon-reload; \
	fi
	@echo "Uninstalled."

clean:
	rm -f $(BINARY)
	rm -rf man/ completions/ dist/
