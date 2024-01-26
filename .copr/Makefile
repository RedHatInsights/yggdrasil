# This Makefile is part of a continuous development workflow integration with
# Fedora's COPR build service (https://copr.fedorainfracloud.org). In order to
# build an RPM in COPR's infrastructure, this Makefile exists to build a
# SRPM. See https://docs.pagure.org/copr.copr/user_documentation.html#scm for
# details on how this Makefile is expected to behave.
#
# N.B.: This Makefile should not be used to build an SRPM manually. Instead,
# see 'dist/srpm' on how to use meson to build an SRPM manually.

outdir ?= $(PWD)/outdir

.PHONY: help
help:
	@echo "This Makefile is not intended to be run manually."
	@echo "Run 'meson compile srpm' instead."

.PHONY: srpm
srpm: deps
	mkdir -p $(outdir)
	meson setup builddir -Dbuild_srpm=True -Dvendor=True -Dexamples=True
	meson compile srpm -C builddir
	mv builddir/dist/srpm/*.src.rpm $(outdir)

deps:
	rpm -qa > /tmp/rpm-list ; \
	grep '^golang-[0-9]' /tmp/rpm-list && \
	grep '^git-core-[0-9]' /tmp/rpm-list && \
	grep '^meson-[0-9]' /tmp/rpm-list && \
	grep '^bash-completion-[0-9]' /tmp/rpm-list && \
	grep '^rpm-build-[0-9]' /tmp/rpm-list && \
	grep '^dbus-1.[0-9]' /tmp/rpm-list && \
	grep '^systemd-[0-9]' /tmp/rpm-list || \
	dnf install -y bash-completion git-core go meson 'pkgconfig(dbus-1)' 'pkgconfig(systemd)' rpm-build
