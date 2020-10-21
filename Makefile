.POSIX:
.SUFFIXES:

# Project variables
PKGNAME := yggdrasil
VERSION := 0.0.1

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
DBUS_INTERFACES_DIR := $(shell pkg-config --variable interfaces_dir dbus-1)
DBUS_SYSTEM_SERVICES_DIR := $(shell pkg-config --variable system_bus_services_dir dbus-1)
DBUS_SYSCONFDIR := $(shell pkg-config --variable sysconfdir dbus-1)
SYSTEMD_SYSTEM_UNIT_DIR := $(shell pkg-config --variable systemdsystemunitdir systemd)

# Build flags
LDFLAGS := 
LDFLAGS += -X github.com/redhatinsights/yggdrasil.Version=$(VERSION)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.PrefixDir=$(PREFIX)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.BinDir=$(BINDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.SbinDir=$(SBINDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.LibexecDir=$(LIBEXECDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.SysconfDir=$(SYSCONFDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.DataDir=$(DATADIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.DatarootDir=$(DATAROOTDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.ManDir=$(MANDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.DocDir=$(DOCDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.LocalstateDir=$(LOCALSTATEDIR)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.DbusInterfacesDir=$(DBUS_INTERFACES_DIR)

BUILDFLAGS :=
ifeq ($(shell find . -name vendor), ./vendor)
BUILDFLAGS += -mod=vendor
endif

BINS = yggd ygg-exec

TARGETS = $(BINS) \
	data/dbus/com.redhat.yggdrasil.service \
	data/systemd/com.redhat.yggd.service

GOSRC := $(shell find . -name '*.go')
GOSRC += go.mod go.sum

build: $(TARGETS)

$(BINS): $(GOSRC)
	go build $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$@

install: build
	pkg-config --modversion dbus-1 || exit 1
	pkg-config --modversion systemd || exit 1
	install -D -m755 ./yggd $(DESTDIR)$(SBINDIR)/yggd
	install -D -m755 ./ygg-exec $(DESTDIR)$(BINDIR)/ygg-exec
	install -D -m644 ./data/dbus/yggdrasil.conf $(DESTDIR)$(DBUS_SYSCONFDIR)/dbus-1/system.d/yggdrasil.conf
	install -D -m644 ./data/dbus/com.redhat.yggdrasil.service $(DESTDIR)$(DBUS_SYSTEM_SERVICES_DIR)/com.redhat.yggdrasil.service
	install -D -m644 ./data/dbus/com.redhat.yggdrasil.xml $(DESTDIR)$(DBUS_INTERFACES_DIR)/com.redhat.yggdrasil.xml
	[[ -e $(DESTDIR)$(SYSCONFDIR)/yggdrasil/config.toml ]] || install -D -m644 ./data/yggdrasil/config.toml $(DESTDIR)$(SYSCONFDIR)/yggdrasil/config.toml
	install -D -m644 ./data/systemd/com.redhat.yggd.service $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/com.redhat.yggd.service

uninstall:
	rm -f $(DESTDIR)$(SBINDIR)/yggd
	rm -f $(DESTDIR)$(BINDIR)/ygg-exec
	rm -f $(DESTDIR)$(DBUS_SYSCONFDIR)/dbus-1/system.d/yggdrasil.conf
	rm -f $(DESTDIR)$(DBUS_SYSTEM_SERVICES_DIR)/com.redhat.yggdrasil.service
	rm -f $(DESTDIR)$(DBUS_INTERFACES_DIR)/com.redhat.yggdrasil.xml
	rm -r $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/com.redhat.yggd.service

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
