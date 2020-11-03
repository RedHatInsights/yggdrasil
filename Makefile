.POSIX:
.SUFFIXES:

# Project variables
SHORTNAME := ygg       # Used as a prefix to binary names. Cannot contain spaces.
LONGNAME  := yggdrasil # Used as file and directory names. Cannot contain spaces.
SUMMARY   := yggdrasil # Used as a long-form description. Can contain spaces and punctuation.
PKGNAME   := yggdrasil # Used as the tarball file name. Cannot contain spaces.
VERSION   := 0.0.1

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
DBUS_INTERFACES_DIR      := $(shell pkg-config --variable interfaces_dir dbus-1)
DBUS_SYSTEM_SERVICES_DIR := $(shell pkg-config --variable system_bus_services_dir dbus-1)
DBUS_SYSCONFDIR          := $(shell pkg-config --variable sysconfdir dbus-1)
SYSTEMD_SYSTEM_UNIT_DIR  := $(shell pkg-config --variable systemdsystemunitdir systemd)

# Build flags
LDFLAGS := 
LDFLAGS += -X github.com/redhatinsights/yggdrasil.Version=$(VERSION)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.ShortName=$(SHORTNAME)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.LongName=$(LONGNAME)
LDFLAGS += -X github.com/redhatinsights/yggdrasil/internal.Summary=$(SUMMARY)
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

BINS = yggd ygg

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
	install -D -m755 ./yggd $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	install -D -m755 ./ygg $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	install -D -m644 ./data/dbus/yggdrasil.conf $(DESTDIR)$(DBUS_SYSCONFDIR)/dbus-1/system.d/$(LONGNAME).conf
	install -D -m644 ./data/dbus/com.redhat.yggdrasil.service $(DESTDIR)$(DBUS_SYSTEM_SERVICES_DIR)/com.redhat.$(LONGNAME).service
	install -D -m644 ./data/dbus/com.redhat.yggdrasil.xml $(DESTDIR)$(DBUS_INTERFACES_DIR)/com.redhat.$(LONGNAME).xml
	[[ -e $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml ]] || install -D -m644 ./data/$(LONGNAME)/config.toml $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml
	install -D -m644 ./data/systemd/yggd.service $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/$(SHORTNAME)d.service

uninstall:
	rm -f $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	rm -f $(DESTDIR)$(BINDIR)/$(SHORTNAME)
	rm -f $(DESTDIR)$(DBUS_SYSCONFDIR)/dbus-1/system.d/$(LONGNAME).conf
	rm -f $(DESTDIR)$(DBUS_SYSTEM_SERVICES_DIR)/com.redhat.$(LONGNAME).service
	rm -f $(DESTDIR)$(DBUS_INTERFACES_DIR)/com.redhat.$(LONGNAME).xml
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
