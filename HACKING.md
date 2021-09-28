# Prerequisites

* _MQTT broker_: An optional MQTT broker, should you wish to publish messages
  locally. `mosquitto` is extremely easy to set up.
* _HTTP server_: An optional HTTP server, should you need to request payloads
  from localhost. This does not need to be more complicated than Python's
  SimpleHTTPServer module.
* `yggd` requires D-Bus and systemd in order to run locally. The header files
  from your distribution's "devel" packages must be installed in order to
  compile `yggd`. A current list of required packages can be found in
  `yggdrasil.spec.in` as listed in the `BuildRequires:` entries.
  * NOTE: These are the packages names are listed as they exist in [Fedora
    Linux](https://getfedora.org/), [CentOS Stream](https://centos.org/), and
    [Red Hat Enterprise
    Linux](https://www.redhat.com/en/technologies/linux-platforms/enterprise-linux).
    Similar package names for other distros may vary.

# Quickstart

Start `yggd`, connecting it to the broker `test.mosquitto.org` over an
unencrypted TCP connection, log judiciously to `stdout`, and bind to the socket
address `@yggd`.


### Terminal 1
```
sudo go run ./cmd/yggd --broker tcp://test.mosquitto.org:8883 --log-level trace --socket-addr @yggd
```

### Terminal 2

```
sudo YGG_SOCKET_ADDR=unix:@yggd go run ./worker/echo
```

### Terminal 3

```
sub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/control/out
```

### Terminal 4

```
yggctl generate control-message --type command '{"command":"ping"}' | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/control/in
```

# Running `yggd`

`yggd` can be compiled using the include `Makefile`, or can be run directly with
the `go run` command. It can read configuration values from a file by running
`yggd` with the `--config` option. A sample configuration file is included in
the `data/yggdrasil` directory.

```
sudo go run ./cmd/yggd --config ./data/yggdrasil/config.toml
```

By default, `yggd` looks for workers in `/usr/local/libexec/yggdrasil`. This
location can be changed by compiling `yggd` with the included `Makefile`, or by
specifying a value for `yggdrasil.LibexecDir` as a linker argument:

```
go run -ldflags "-X 'github.com/redhatinsights/yggdrasil.LibexecDir=/usr/libexec/yggdrasil'" ./cmd/yggd
```

Many default paths (such as Prefix, BinDir, LocalstateDir, etc), as well as some
other compile-time constants, can be specified by providing a linker `-X` flag
argument. See the `Makefile` or `constants.go` for a complete list. 

# Useful Utilities

## `yggctl`

`yggctl` is a program that can interact with a running `yggd` process over an
RPC interface. It is currently very limited in its functionality. Until this
program provides usefulness to users, rather than just to developers, it will
not be installed by default. It can be run directly with `go run`, or installed
with `go install`.

See the output of `yggctl --help` for basic commands, or `yggctl --help-full`
for a complete list of available commands.


## `worker/echo`

`echo` is a very simple reference implementation for a gRPC-based worker written
in Go. 

If you ran `yggd` with a specified `--socket-addr` value, you can connect the
`echo` worker directly to your running `yggd` process by specifying the
`YGG_SOCKET_ADDR` environment variable:

```
sudo YGG_SOCKET_ADDR=unix:@yggd go run ./worker/echo
```

Alternatively, you can compile the echo worker and install it into `yggd`'s
worker directory (defaults to `/usr/local/libexec/yggdrasil` unless overridden
at compile time).

```
go build -o echo-worker ./worker/echo
sudo install -D -m 755 echo-worker /usr/local/libexec/yggdrasil/
```

## `mqttcli`

[`mqttcli`](https://git.sr.ht/~spc/mqttcli) is a separate program that is useful
for publishing messages and subscribing to topics on an MQTT broker. `mqttcli`
can be installed with `go install`:

```
go install git.sr.ht/~spc/mqttcli/cmd/...
```

Or if you're running Fedora 34 or later, it can be installed directly with
`dnf`:

```
dnf install mqttcli
```

# Sending Data

With a running `yggd` and `echo` worker, it should be possible to publish a
message to the broker, destined for one of the topics `yggd` is subscribed to.

## Monitoring topics

To watch a topic for messages, subscribe to it with `sub`:

```
sub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/data/in` -topic yggdrasil/$CLIENT_ID/data/out -topic yggdrasil/$CLIENT_ID/control/out
```

## Publish a message

A client can be sent a `PING` command by generated a control message and
publishing it to the client's "control/in" topic:

```
yggctl generate control-message --type command '{"command":"ping"}' | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/control/in
```

Activity should occur on the terminal attached to `yggd`, and a `PONG` event
message should be received on the "control/out" topic, subscribed to in
[Monitoring topics](#Monitoring topics).

Similarly, a data message can be published to a client using `yggctl generate`
and `pub`.

```
yggctl generate data-message --directive echo hello | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/data/in
```

# Call Graphs

Call graphs can be generated to provide a high-level overview of the
interactions between packages.

For basic call graphs, install `go-callvis` (`go get -u
github.com/ofabry/go-callvis`) and run:

```bash
# Call graph of the main function of yggd, up to calls into the yggdrasil package
go-callvis -nostd -format png -file yggdrasil.main ./cmd/yggd
# Call graph of the yggdrasil package, as invoked by yggd
go-callvis -nostd -format png -file yggdrasil.yggdrasil -focus github.com/redhatinsights/yggdrasil ./cmd/yggd
```

For more detailed, interactive call graphs, install `callgraph` and `digraph`.

```bash
go get -u golang.org/x/tools/cmd/callgraph
go get -u golang.org/x/tools/cmd/digraph
```

Generate a call graph using `callgraph`, filter the resulting graph to exclude
standard library calls and pipe the result into `digraph`. See the `-help`
output of `digraph` for how to interact with the graph.

```bash
callgraph -algo pta -format digraph ./cmd/yggd | grep github.com/redhatinsights/yggdrasil | sort | uniq | digraph
```

# Code Guidelines

* Commits follow the [Conventional Commits](https://www.conventionalcommits.org)
  pattern.
* Communicate errors through return values, not logging. Library functions in
  particular should follow this guideline. You never know under which condition
  a library function will be called, so excessive logging should be avoided.

# Contact Us

Chat on Matrix: [#yggd:matrix.org](https://matrix.to/#/#yggd:matrix.org).
