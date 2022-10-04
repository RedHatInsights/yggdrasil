[![godocs.io](https://godocs.io/github.com/RedHatInsights/yggdrasil?status.svg)](https://godocs.io/github.com/RedHatInsights/yggdrasil)

# yggdrasil

_yggdrasil_ is a system daemon that subscribes to topics on an MQTT broker and
routes any data received on the topics to an appropriate child "worker" process,
exchanging data with its worker processes through a D-Bus message broker.

## Installation

The easiest way to install yggdrasil is by using the include `Makefile`. Because
yggdrasil runs as a privileged system daemon, systemd unit files and D-Bus
policy files must be installed in specific directories in order for the service
to start.

The default target will build all _yggdrasil_ binaries and ancillary data files.
The `Makefile` also includes an `install` target to install the binaries and
data into distribution-appropriate locations. To override the installation
directory (commonly referred to as the `DESTDIR`), set the `DESTDIR` variable
when running the `install` target. Additional variables can be used to further
configure the installation prefix and related directories.

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

Any of these variables can be overridden by passing a value to `make`. For
example:

```bash
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var DESTDIR=/tmp/rpmbuildroot install
```

### Branding

_yggdrasil_ can be rebranded by setting some additional `make` variables:

```
SHORTNAME := ygg       # Used as a prefix to binary names. Cannot contain spaces.
LONGNAME  := yggdrasil # Used as file and directory names. Cannot contain spaces.
SUMMARY   := yggdrasil # Used as a long-form description. Can contain spaces and punctuation.
```

For example, to brand _yggdrasil_ as `bunnies`, compile as follows:

```bash
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var SHORTNAME=bnns LONGNAME=bunnies SUMMARY="Bunnies have a way of proliferating." install
```

This will build `yggd`, but install it into `DESTDIR` as `bnnsd`. Accordingly,
the systemd service will be named `bunnies.service` with a `Description=`
directive of "Bunnies have a way of proliferating.".

## Configuration

Configuration of `yggd` can be done by specifying values in a configuration file
or via command line arguments. Command-line arguments take precedence over
configuration file values. The configuration file is [TOML](https://toml.io).

The system-wide configuration file is located at `/etc/yggdrasil/config.toml`
(assuming `SYSCONFDIR=/etc`, as the example above). The location of the file may
be overridden by passing the `--config` command-line argument to `yggd`.

### (Optional) Authentication

In order to run `yggd` under certain conditions (such as connecting to a broker
that requires mTLS authentication), a valid certificate must first be created
and written to the filesystem.

#### Red Hat Subscription Manager

One way of generating a valid certificate is to first register the system with
an RHSM provider. The simplest way to do this is to create a free [Red Hat
Developer account](https://developers.redhat.com/register). On a Red Hat
Enterprise Linux system, run `subscription-manager register`, using the
developer account username and password.

```
sudo subscription-manager register --username j_developer@company.com --password sw0rdf1sh
```

Once the system is successfully registered with RHSM, `yggd` can be launched,
using the certificate key pair:

```
sudo /usr/sbin/yggd --cert-file /etc/pki/consumer/cert.pem --key-file /etc/pki/consumer/key.pem
```

### Tags

A set of tags may be defined to associate additional key/value data with a host
when connecting to the broker. To do this, create the file
`/etc/yggdrasil/tags.toml` (assuming `SYSCONFDIR=/etc`, as the example above).
The contents of the file may be any number of TOML key/value pairs. However, a
limited number of TOML values are accepted as tag values (strings, integers,
booleans, floats, Local Date, Local Time, Offset Date-Time and Local Date-Time).

## Running

yggdrasil uses D-Bus as an IPC framework to enable communication between workers
and the dispatcher. `yggd` can run on either the system bus (the default) or a
session bus. Connecting to a session bus is enabled either implicitly by setting
the `DBUS_SESSION_BUS_ADDRESS` environment variable or explicitly by running
`yggd` with the `--bus-address` flag. In either case, a D-Bus policy is
installed that denies non-root D-Bus clients from sending messages to both the
dispatcher and the workers.

The systemd unit `yggd.service` starts `yggd`, connecting it to the system bus.

```
systemctl enable --now yggd
```

Multiple instances of `yggd` can be run by using the templated systemd units.
Each instance requires  a private D-Bus session. The socket and broker are
started with `yggdrasil-bus@.socket` and `yggdrasil-bus@.service`, respectively.
A templated yggd unit is available to automatically connect yggd to the D-Bus
broker started by `yggdrasil-bus@.service`.

```
systemctl enable --now yggdrasil-bus@bunnies.socket
systemctl enable --now yggdrasil-bus@bunnies.service
systemctl enable --now yggd@bunnies.service
```

This will define and create an abstract UNIX domain socket named `yggd_bunnies`.
It will start a `dbus-broker` process running in the "user" scope, connecting to
the `yggd_bunnies` socket defined previously. Finally, `yggd` is launched with a
specific configuration file as the value of the `--config` argument:
`/etc/yggdrasil/yggdrasil-bunnies.toml`.

## Workers

In order for `yggd` to locate and launch a worker, a configuration file must be
installed into the `workerconfdir`. This path is defined at compile time as
`$SYSCONFDIR/$LONGNAME/workers`. The name of this file, trimming the `.toml`
suffix, is known as the worker "directive". This value must be unique among all
workers connected to a given dispatcher and message bus. This value is the
destination name through which a worker is uniquely identified when data is
received by `yggd`.

_yggdrasil_ includes a pkg-config module named "yggdrasil", enabling worker
developers to look up the worker config directory at build time:

```
$ pkg-config --variable workerconfdir yggdrasil
/etc/yggdrasil/workers
```

The following table includes valid fields for a worker configuration file:

| **Field**        | **Value** | **Description** |
| ----------       | --------- | --------------- |
| `exec`           | `string`  | Path to an executable that is assumed to be the worker program (required). |
| `env`            | `array`   | Any additional values that a worker needs injected into its runtime enviroment before starting up. `PATH` and all variables beginning with `YGG_` are forbidden and may not be overridden. |
| `remote_content` | `bool`    | A `true` value assumes that content needs to be downloaded from a remote URL before being passed to the worker. |
| `features`       | `table`   | An arbitrary set of key/value pairs that are included in connection-status messages sent by the dispatcher. |

An example of a worker configuration file can be see in the example `echo`
worker directory: `./workers/echo/config.toml`.

### Worker Programs

A functional worker program must connect to the message bus as determined by the
`DBUS_SESSION_BUS_ADDRESS` environment variable, defaulting to the system bus if
the variable is empty or undefined. Once connected to the bus:

* The program must export an object on the bus that implements the
  `com.redhat.yggdrasil.Worker1` interface.
* The object must be exported at a path under  `/com/redhat/yggdrasil/Worker1`
  that includes the directive name (i.e. `/com/redhat/yggdrasil/Worker1/echo`).
* The worker must claim a well-known name that begins with
  `com.redhat.yggdrasil.Worker1` and includes its directive as the final segment
  in reverse-domain-name notation (i.e. `com.redhat.yggdrasil.Worker1.echo`).

A worker can transmit data back to a destination by calling the
`com.redhat.yggdrasil.Dispatcher1.Transmit` method.

See `worker/echo` for a working implementation of a worker program.
