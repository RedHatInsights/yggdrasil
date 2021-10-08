include Makefile

SOURCEDIR := $(shell rpmbuild -E %_sourcedir)
SRCRPMDIR := $(shell rpmbuild -E %_srcrpmdir)
$(shell mkdir -p $(SOURCEDIR))
$(shell mkdir -p $(SRCRPMDIR))

srpm: SHORTNAME = $(shell basename $(PWD))
srpm: LONGNAME = $(SHORTNAME)
srpm: BRANDNAME = $(SHORTNAME)
srpm: VERSION := $(shell echo $(VERSION) | awk -F. '{printf("%d.%d.%d", $$1, $$2, $$3+1)}')
srpm: RELEASE = $(shell printf "0.%s.git.%s" $(shell git rev-list $(shell git describe --tags --abbrev=0 | tr -d '\n')..HEAD --count | tr -d '\n') $(shell git rev-parse --short HEAD | tr -d '\n'))
srpm: PKGNAME = $(SHORTNAME)

ifeq ($(shell basename $(PWD)), rhc)
srpm: TOPICPREFIX = redhat/insights
srpm: DATAHOST = cert.cloud.redhat.com
srpm: PROVIDER = Red Hat
else
srpm: TOPICPREFIX = $(SHORTNAME)
srpm: DATAHOST = ""
endif

.PHONY: srpm
srpm: deps
	sed \
	    -e 's,[@]SHORTNAME[@],$(SHORTNAME),g' \
		-e 's,[@]LONGNAME[@],$(LONGNAME),g' \
		-e 's,[@]BRANDNAME[@],$(BRANDNAME),g' \
		-e 's,[@]VERSION[@],$(VERSION),g' \
		-e 's,[@]RELEASE[@],$(RELEASE),g' \
		-e 's,[@]PKGNAME[@],$(PKGNAME),g' \
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
		-e 's,[@]ARCH[@],$(ARCH),g' \
		yggdrasil.spec.in > yggdrasil.spec.tmp && mv yggdrasil.spec.tmp yggdrasil.spec
	make PKGNAME=$(PKGNAME) VERSION=$(VERSION)-$(RELEASE) dist
	install -D -m644 $(PKGNAME)-$(VERSION)-$(RELEASE).tar.gz $(SOURCEDIR)/
	rpmbuild -bs $(spec)/yggdrasil.spec
	install -D -m644 $(SRCRPMDIR)/*.rpm $(outdir)/

deps:
	dnf install -y golang git
