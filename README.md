[![godocs.io](https://godocs.io/github.com/RedHatInsights/yggdrasil?status.svg)](https://godocs.io/github.com/RedHatInsights/yggdrasil)

# yggdrasil

_yggdrasil_ is a system daemon that subscribes to topics on an MQTT broker and
routes any data received on the topics to an appropriate child "worker" process,
exchanging data with its worker processes through a D-Bus message broker.

## Installation

The easiest way to compile and install yggdrasil is using `meson`. Because
yggdrasil runs as a privileged system daemon, systemd unit files and D-Bus
policy files must be installed in specific directories in order for the service
to start.

Generally, it is recommended to follow your distribution's packaging guidelines
for compiling Go programs and installing projects using `meson`. What follows is
a generally acceptable set of steps to setup, compile and install yggdrasil
using `meson`.

```bash
# Set up the project according to distribution-specific directory locations
meson setup --prefix /usr/local --sysconfdir /etc --localstatedir /var builddir
# Compile
meson compile -C builddir
# Install
meson install -C builddir
```

`meson` includes an optional `--destdir` to its `install` subcommand to aid in
packaging.

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
and the dispatcher, as well as exporting an API enabling other services to
interact with it. `yggd` follows the normal logic for determining which bus to
connect to. It will attempt to connect to the session bus if
`DBUS_SESSION_BUS_ADDRESS` is defined. Otherwise, it will attempt to connect to
the system bus. On the system bus, if the process is not running as root, the
installed D-Bus security policy will deny the process from claiming the
`com.redhat.Yggdrasil1` and `com.redhat.yggdrasil.Dispatcher1` names, and the
process will exit.

The systemd unit `yggrasil.service` starts `yggd`, using the logic described
above.

```
systemctl enable --now yggdrasil
```

Multiple instances of `yggd` can be run by using the templated systemd units.
Each instance requires  a private D-Bus session. The socket and broker are
started with `yggdrasil-bus@.socket` and `yggdrasil-bus@.service`, respectively.
A templated yggdrasil unit is available to automatically connect yggd to the
D-Bus broker started by `yggdrasil-bus@.service`.

```
systemctl enable --now yggdrasil-bus@bunnies.socket
systemctl enable --now yggdrasil-bus@bunnies.service
systemctl enable --now yggdrasil@bunnies.service
```

This will define and create an abstract UNIX domain socket named `yggd_bunnies`.
It will start a `dbus-broker` process running in the "user" scope, connecting to
the `yggd_bunnies` socket defined previously. Finally, `yggd` is launched with a
specific configuration file as the value of the `--config` argument:
`/etc/yggdrasil/yggdrasil-bunnies.toml`.

## Workers

A functional worker program must connect to the message bus as determined by the
`DBUS_SESSION_BUS_ADDRESS` environment variable, connecting to a session bus if
the value is defined, otherwise connecting to the system bus. Once connected to
the bus:

* The program must export an object on the bus that implements the
  `com.redhat.yggdrasil.Worker1` interface.
* The object must be exported at a path under  `/com/redhat/yggdrasil/Worker1`
  that includes the directive name (i.e. `/com/redhat/yggdrasil/Worker1/echo`).
* The worker must claim a well-known name that begins with
  `com.redhat.yggdrasil.Worker1` and includes its directive as the final segment
  in reverse-domain-name notation (i.e. `com.redhat.yggdrasil.Worker1.echo`).

A worker can transmit data back to a destination by calling the
`com.redhat.yggdrasil.Dispatcher1.Transmit` method.

Package `worker` implements the above requirements implicitly, enabling workers
to be written without needing to worry about much of the D-Bus requirements
outlined above.

See `worker/echo` for a reference implementation of a worker program.
