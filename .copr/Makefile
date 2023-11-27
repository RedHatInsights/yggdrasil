# This Makefile is part of a continuous development workflow integration with
# Fedora's COPR build service (https://copr.fedorainfracloud.org). In order to
# build an RPM in COPR's infrastructure, this Makefile exists to build a
# SRPM. See https://docs.pagure.org/copr.copr/user_documentation.html#scm for
# details on how this Makefile is expected to behave.
#
# N.B.: This Makefile should not be used to build an SRPM manually. Instead,
# see 'dist/srpm' on how to use meson to build an SRPM manually.

.PHONY: help
help:
	@echo "This Makefile is not intended to be run manually."
	@echo "Run 'meson compile srpm' instead."

.PHONY: srpm
srpm:
	dnf install --assumeyes bash-completion git-core go meson 'pkgconfig(dbus-1)' 'pkgconfig(systemd)' rpm-build
	meson setup builddir -Dbuild_srpm=True -Dvendor=True -Dexamples=True
	meson compile srpm -C builddir
	mv builddir/dist/srpm/*.src.rpm $(outdir)
