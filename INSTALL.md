## Basic Installation

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

## Branding

`yggdrasil` can be rebranded by setting some additional `make` variables:

```
SHORTNAME := ygg       # Used as a prefix to binary names. Cannot contain spaces.
LONGNAME  := yggdrasil # Used as file and directory names. Cannot contain spaces.
SUMMARY   := yggdrasil # Used as a long-form description. Can contain spaces and punctuation.
```

For example, to brand `yggdrasil` as `bunnies`, compile as follows:

```bash
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var SHORTNAME=bnns LONGNAME=bunnies SUMMARY="Bunnies have a way of proliferating." install
```

This will build `yggd` and `ygg`, but install them into `DESTDIR` as `bnnsd`
and `bnns`, respectively. Accordingly, the systemd service will be named
`bunnies.service` with a `Description=` directive of "Bunnies have a way of proliferating.".
