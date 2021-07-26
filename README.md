[![godocs.io](https://godocs.io/github.com/RedHatInsights/yggdrasil?status.svg)](https://godocs.io/github.com/RedHatInsights/yggdrasil)

# yggdrasil

yggdrasil is a client daemon that establishes a receiving queue for instructions
to be sent to the system via a broker.

## `./cmd/yggd`

`yggd` is a daemon that connects to an MQTT broker, subscribes to a pair of
topics and dispatches messages to an appropriate worker subprocess.

# Getting Started

## Install

```
export MAKE_FLAGS="PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var"
make ${MAKE_FLAGS}
sudo make ${MAKE_FLAGS} install
```

## Register/Activate

In order to run `yggd`, a system must first be registered with an RHSM provider.
The simplest way to do this is to create a free [Red Hat Developer
account](https://developers.redhat.com/register). On a Red Hat Enterprise Linux
system, run `subscription-manager register`, using the developer account
username and password.

```
sudo subscription-manager register --username j_developer@company.com --password sw0rdf1sh
```

This will register the system with RHSM. `yggd` can then be activated using
`systemctl`.

```
sudo systemctl enable --now yggd
```

# Configuration

Configuration of `yggd` can be done by specifying values in a configuration file
or via command line arguments. Command-line arguments take precendence over
configuration file values. The configuration file is [TOML](https://toml.io).

The system-wide configuration file is located at `/etc/yggdrasil/config.toml` 
(assuming `SYSCONFDIR=/etc`, as the example above). The location of the file may
be overridden by passing the `--config` command-line argument to `yggd`.

# Tags

A set of tags may be defined to associate additional key/value data with a host
when connecting to the broker. To do this, create the file
`/etc/yggdrasil/tags.toml` (assuming `SYSCONFDIR=/etc`, as the example above).
The contents of the file may be any number of TOML key/value pairs. However, a
limited number of TOML values are accepted as tag values (strings, integers,
booleans, floats, Local Date, Local Time, Offset Date-Time and Local Date-Time).

# Workers

`yggd` has a dispatcher/worker protocol defined in `protocol/yggdrasil.proto`.
This protocol defines two services (`Dispatcher` and `Worker`), along with the
messages necessary for the services to exchange data. In order to interact with
`yggd` as a worker process an executable must be installed into
`$LIBEXECDIR/yggdrasil/`. The executable name must end with the string "worker".
A pkg-config module named 'yggdrasil' is installed so that workers can locate
the worker exec directory at build time.

```
pkg-config --variable workerexecdir yggdrasil
/usr/local/libexec/yggdrasil
```

A worker program must implement the `Worker` service.  `yggd` will execute
worker programs at start up. It will set the `YGG_SOCKET_ADDR` variable in the
worker's environment. This address is the socket on which the worker must dial
the dispatcher and call the "Register" RPC method. Upon successful registration,
the worker will receive back a socket address. The worker must bind to and
listen on this address for RPC methods. See `worker/echo` for an example worker
process that does nothing more than return the content data it received from the
dispatcher.
