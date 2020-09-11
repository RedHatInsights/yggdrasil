.POSIX:
.SUFFIXES:

PKGNAME = yggdrasil
VERSION = $(shell git describe --tags --always --dirty --match=v*)

PREFIX        ?= /usr/local
BINDIR        ?= $(PREFIX)/bin
SBINDIR       ?= $(PREFIX)/sbin
LIBEXECDIR    ?= $(PREFIX)/libexec
SYSCONFDIR    ?= $(PREFIX)/etc
DATADIR       ?= $(PREFIX)/share
DATAROOTDIR   ?= $(PREFIX)/share
MANDIR        ?= $(DATADIR)/man
DOCDIR        ?= $(PREFIX)/doc
LOCALSTATEDIR ?= $(PREFIX)/var

GO      ?= go
RM      ?= rm -f
MV      ?= mv
INSTALL ?= install
SED     ?= sed

GOFLAGS ?= 

GOSRC != find . -name '*.go'
GOSRC += go.mod go.sum

TARGETS = yggd ygg-exec data/dbus/com.redhat.yggdrasil1.service

GOFLAGS += -ldflags "\
	-X github.com/redhatinsights/yggdrasil/pkg.Version=$(VERSION) \
	-X github.com/redhatinsights/yggdrasil/internal.PrefixDir=$(PREFIX) \
	-X github.com/redhatinsights/yggdrasil/internal.BinDir=$(BINDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.SbinDir=$(SBINDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.LibexecDir=$(LIBEXECDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.SysconfDir=$(SYSCONFDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.DataDir=$(DATADIR) \
	-X github.com/redhatinsights/yggdrasil/internal.DatarootDir=$(DATAROOTDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.ManDir=$(MANDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.DocDir=$(DOCDIR) \
	-X github.com/redhatinsights/yggdrasil/internal.LocalstateDir=$(LOCALSTATEDIR)"
build: $(TARGETS)

yggd: $(GOSRC)
	$(GO) build $(GOFLAGS) ./cmd/yggd

ygg-exec: $(GOSRC)
	$(GO) build $(GOFLAGS) ./cmd/ygg-exec

install: build
	$(INSTALL) -D -m755 ./yggd $(DESTDIR)$(SBINDIR)/yggd
	$(INSTALL) -D -m755 ./ygg-exec $(DESTDIR)$(BINDIR)/ygg-exec
	$(INSTALL) -D -m644 ./data/dbus/yggdrasil.conf $(DESTDIR)$(SYSCONFDIR)/dbus-1/system.d/yggdrasil.conf
	$(INSTALL) -D -m644 ./data/dbus/com.redhat.yggdrasil1.service $(DESTDIR)$(DATADIR)/dbus/services/com.redhat.yggdrasil1.service

%: %.in Makefile
	$(SED) \
		-e 's,[@]VERSION[@],$(VERSION),g' \
		-e 's,[@]PACKAGE[@],$(PACKAGE),g' \
		-e 's,[@]PREFIX[@],$(PREFIX),g' \
		-e 's,[@]BINDIR[@],$(BINDIR),g' \
		-e 's,[@]SBINDIR[@],$(SBINDIR),g' \
		-e 's,[@]LIBEXECDIR[@],$(LIBEXECDIR),g' \
		-e 's,[@]DATAROOTDIR[@],$(DATAROOTDIR),g' \
		-e 's,[@]DATADIR[@],$(DATADIR),g' \
		-e 's,[@]SYSCONFDIR[@],$(SYSCONFDIR),g' \
		-e 's,[@]LOCALSTATEDIR[@],$(LOCALSTATEDIR),g' \
		-e 's,[@]DOCDIR[@],$(DOCDIR),g' \
		$< > $@.tmp && $(MV) $@.tmp $@

clean:
	$(GO) mod tidy
	$(RM) $(TARGETS)
	
.PHONY: build clean install
