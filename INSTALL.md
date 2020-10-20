`yggdrasil` includes a `Makefile` to aid distributions in packaging. The default
target will build all `yggdrasil` binaries and ancillary data files. The
`Makefile` also includes an `install` target to install the binaries and data
into distribution-appropriate locations. To override the installation directory
(commonly referred to as the `DESTDIR`), set the `DESTDIR` variable when running
the `install` target. Additional variables can be used to further configure the
installation prefix and related directories.

```
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
```

Any of these variables can be overriden by passing a value to `make`. For
example:

```bash
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var DESTDIR=/tmp/rpmbuildroot install
```

# Detailed steps how to install on RHEL 7 machine

```
yum install golang dbus-devel.x86_64
git clone https://github.com/RedHatInsights/yggdrasil
cd yggdrasil
make
./yggd --help
./ygg-exec --help
make install
```
