SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
.POSIX:
.SUFFIXES:

# Project variables

# Used as a prefix to binary names. Cannot contain spaces.
SHORTNAME := ygg
# Used as file and directory names. Cannot contain spaces.
LONGNAME  := yggdrasil
# Used as a long-form description. Can contain spaces and punctuation.
BRANDNAME   := yggdrasil
# Used as the tarball file name. Cannot contain spaces.
PKGNAME   := yggdrasil
VERSION   := 0.2.98
# Used as the prefix for MQTT topic names
TOPICPREFIX := yggdrasil
# Used to force sending all HTTP traffic to a specific host.
DATAHOST := 
# Used to identify the agency providing the connection broker.
PROVIDER :=

# Installation directories
PREFIX        ?= /usr/local
BINDIR        ?= $(PREFIX)/bin
SBINDIR       ?= $(PREFIX)/sbin
LIBEXECDIR    ?= $(PREFIX)/libexec
SYSCONFDIR    ?= $(PREFIX)/etc
DATADIR       ?= $(PREFIX)/share
DATAROOTDIR   ?= $(PREFIX)/share
MANDIR        ?= $(DATADIR)/man
DOCDIR        ?= $(DATADIR)/doc
LOCALSTATEDIR ?= $(PREFIX)/var
DESTDIR       ?=

# Dependent package directories
SYSTEMD_SYSTEM_UNIT_DIR  := $(shell pkg-config --variable systemdsystemunitdir systemd)

# Build flags
LDFLAGS := 
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.Version=$(VERSION)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.ShortName=$(SHORTNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LongName=$(LONGNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.BrandName=$(BRANDNAME)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.PrefixDir=$(PREFIX)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.BinDir=$(BINDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.SbinDir=$(SBINDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LibexecDir=$(LIBEXECDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.SysconfDir=$(SYSCONFDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DataDir=$(DATADIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DatarootDir=$(DATAROOTDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.ManDir=$(MANDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DocDir=$(DOCDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.LocalstateDir=$(LOCALSTATEDIR)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.TopicPrefix=$(TOPICPREFIX)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.DataHost=$(DATAHOST)'
LDFLAGS += -X 'github.com/redhatinsights/yggdrasil.Provider=$(PROVIDER)'

BUILDFLAGS ?=
BUILDFLAGS += -buildmode=pie
ifeq ($(shell find . -name vendor), ./vendor)
BUILDFLAGS += -mod=vendor
endif

BINS = yggd
DATA = yggd.bash \
	   yggd.1.gz \
	   yggd-USAGE.md \
	   data/systemd/yggd.service \
	   data/pkgconfig/yggdrasil.pc \
	   doc/tags.toml

GOSRC := $(shell find . -name '*.go')
GOSRC += go.mod go.sum

.PHONY: all
all: $(BINS) $(DATA)

.PHONY: bin
bin: $(BINS)

$(BINS): $(GOSRC)
	go build $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$@

.PHONY: data
data: $(DATA)

%.bash: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$(patsubst %.bash,%,$@) --generate-bash-completion > $@

%.1: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$(patsubst %.1,%,$@) --generate-man-page > $@

%.1.gz: %.1
	gzip -k $^

%-USAGE.md: $(GOSRC)
	go run $(BUILDFLAGS) -ldflags "$(LDFLAGS)" ./cmd/$(patsubst %-USAGE.md,%,$@) --generate-markdown > $@

%: %.in Makefile
	sed \
	    -e 's,[@]SHORTNAME[@],$(SHORTNAME),g' \
		-e 's,[@]LONGNAME[@],$(LONGNAME),g' \
		-e 's,[@]BRANDNAME[@],$(BRANDNAME),g' \
		-e 's,[@]VERSION[@],$(VERSION),g' \
		-e 's,[@]PACKAGE[@],$(PACKAGE),g' \
		-e 's,[@]TOPICPREFIX[@],$(TOPICPREFIX),g' \
		-e 's,[@]DATAHOST[@],$(DATAHOST),g' \
		-e 's,[@]PROVIDER[@],$(PROVIDER),g' \
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

.PHONY: install
install: $(BINS) $(DATA)
	pkg-config --modversion dbus-1 || exit 1
	pkg-config --modversion systemd || exit 1
	install -D -m755 ./yggd $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	[[ -e $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml ]] || install -D -m644 ./data/yggdrasil/config.toml $(DESTDIR)$(SYSCONFDIR)/$(LONGNAME)/config.toml
	install -D -m644 ./data/systemd/yggd.service $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/$(SHORTNAME)d.service
	install -D -m644 ./yggd.1.gz $(DESTDIR)$(MANDIR)/man1/$(SHORTNAME)d.1.gz
	install -D -m644 ./yggd.bash $(DESTDIR)$(DATADIR)/bash-completion/completions/$(SHORTNAME)d
	install -D -m644 ./data/pkgconfig/yggdrasil.pc $(DESTDIR)$(PREFIX)/share/pkgconfig/$(LONGNAME).pc
	install -d -m755 $(DESTDIR)$(LIBEXECDIR)/$(LONGNAME)

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(SBINDIR)/$(SHORTNAME)d
	rm -r $(DESTDIR)$(SYSTEMD_SYSTEM_UNIT_DIR)/$(SHORTNAME)d.service
	rm -f $(DESTDIR)$(MANDIR)/man1/$(SHORTNAME)d.1.gz
	rm -f $(DESTDIR)$(DATADIR)/bash-completion/completions/$(SHORTNAME)d
	rm -f $(DESTDIR)$(PREFIX)/share/pkgconfig/$(LONGNAME).pc

.PHONY: dist
dist:
	go mod vendor
	tar --create \
		--gzip \
		--file /tmp/$(PKGNAME)-$(VERSION).tar.gz \
		--exclude=.git \
		--exclude=.vscode \
		--exclude=.github \
		--exclude=.gitignore \
		--exclude=.copr \
		--transform s/^\./$(PKGNAME)-$(VERSION)/ \
		. && mv /tmp/$(PKGNAME)-$(VERSION).tar.gz .
	rm -rf ./vendor

.PHONY: clean
clean:
	go mod tidy
	rm $(BINS)
	rm $(DATA)
