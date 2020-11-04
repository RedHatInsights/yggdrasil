.POSIX:
.SUFFIXES:

# Project variables

# Used as a prefix to binary names. Cannot contain spaces.
SHORTNAME := ygg
# Used as file and directory names. Cannot contain spaces.
LONGNAME  := yggdrasil
# Used as a long-form description. Can contain spaces and punctuation.
SUMMARY   := yggdrasil
# Used as the tarball file name. Cannot contain spaces.
PKGNAME   := yggdrasil
VERSION   := 0.0.1

# Compile-time constants
BROKERADDR := tcp://localhost:1883

# Installation directories
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

# Dependent package directories
SYSTEMD_SYSTEM_UNIT_DIR  := $(shell pkg-config --variable systemdsystemunitdir systemd)

# Build flags
LDFLAGS := 
LDFLAGS += -X github.com/redhatinsights/yggdrasil.Version=$(VERSION)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.ShortName=$(SHORTNAME)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.LongName=$(LONGNAME)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.Summary=$(SUMMARY)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.BrokerAddr=$(BROKERADDR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.PrefixDir=$(PREFIX)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.BinDir=$(BINDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.SbinDir=$(SBINDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.LibexecDir=$(LIBEXECDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.SysconfDir=$(SYSCONFDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.DataDir=$(DATADIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.DatarootDir=$(DATAROOTDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.ManDir=$(MANDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.DocDir=$(DOCDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil.LocalstateDir=$(LOCALSTATEDIR)

BUILDFLAGS :=
ifeq ($(shell find . -name vendor), ./vendor)
BUILDFLAGS += -mod=vendor
endif

BINS = yggd ygg

TARGETS = $(BINS) \
	data/systemd/yggd.service

GOSRC := $(shell find . -name '*.go')
GOSRC += go.mod go.sum

build: $(TARGETS)

$(BINS): $(GOSRC)
	go build $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$@

install: build
	pkg-config --modversion dbus-1 || exit 1
	pkg-config --modversion systemd || exit 1
	install -D -m755 ./yggd $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	install -D -m755 ./ygg $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	[[ -e $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml ]] || install -D -m644 ./data/$(LONGNAME)/config.toml $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml
	install -D -m644 ./data/systemd/yggd.service $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/$(SHORTNAME)d.service

uninstall:
	rm -f $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	rm -f $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	rm -r $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/$(SHORTNAME)d.service

dist:
	go mod vendor
	tar --create \
		--gzip \
		--file /tmp/$(PKGNAME)-$(VERSION).tar.gz \
		--exclude=.git \
		--exclude=.vscode \
		. && mv /tmp/$(PKGNAME)-$(VERSION).tar.gz .
	rm -rf ./vendor

%: %.in Makefile
	sed \
	    -e 's,[@]SHORTNAME[@],$(SHORTNAME),g' \
		-e 's,[@]LONGNAME[@],$(LONGNAME),g' \
		-e 's,[@]SUMMARY[@],$(SUMMARY),g' \
		-e 's,[@]BROKERADDR[@],$(BROKERADDR),g' \
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
		$< > $@.tmp && mv $@.tmp $@

clean:
	go mod tidy
	rm $(TARGETS)
	
.PHONY: build clean install uninstall
