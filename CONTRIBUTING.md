# Prerequisites

## MQTT broker

An optional MQTT broker, should you wish to publish messages  locally.
`mosquitto` is extremely easy to set up.

## HTTP server

An optional HTTP server, should you need to request payloads from localhost.
This does not need to be more complicated than Python's SimpleHTTPServer module.

## D-Bus

`yggd` requires D-Bus and systemd in order to run locally. The header files from
your distribution's "devel" packages must be installed in order to compile
`yggd`. A current list of required packages can be found in the top-level
[`meson.build`](https://github.com/RedHatInsights/yggdrasil/blob/main/meson.build)
file. The package names will vary depending on your distribution.

## MQTT client

[`mqttcli`](#mqttcli) is recommended to make use of the `pub` and `sub`
programs.

# Quickstart

### Terminal 1

Start `yggd` on the user's session bus, connecting it to the broker
`test.mosquitto.org` over an unencrypted TCP connection, log judiciously to
`stdout`.

```
go run ./cmd/yggd --server tcp://test.mosquitto.org:8883 --log-level trace --client-id $(hostname)
```

### Terminal 2

Start an `echo` worker.

```
go run ./worker/echo -log-level trace
```

### Terminal 3

Subscribe to all the MQTT topics the `yggd` client will publish and subscribe to
to monitor the MQTT traffic.

```
sub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$(hostname)/#
```

### Terminal 4

Publish a "PING" command to the `yggd` "control/in" topic.

```
go run ./cmd/yggctl generate control-message --type command '{"command":"ping"}' | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$(hostname)/control/in
```

You should see the message logged to the output of `sub` in [Terminal
3](#terminal-3) and receipt of the message logged in the output of `yggd` in
[Terminal 1](#terminal-1).

Now publish a data message to the echo worker.

```
go run ./cmd/yggctl generate data-message --directive worker "hello" | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$(hostname)/data/in
```

Again, you should see the message logged by the `sub` command in [Terminal
3](#terminal-3), the receipt of the message logged in the output of `yggd` in
[Terminal 1](#terminal-1). This time, you should see output in [Terminal
2](#terminal-2) when the echo worker receives the message.

## Running `yggd` on the system bus

This quickstart assumes yggdrasil will communicate with workers over the user's
private session D-Bus service. To run `yggd` on the system bus, install the
D-Bus security policy allowing root to own the appropriate names on the system
bus.

```
install -D -m644 ./data/dbus/yggd.conf /usr/share/dbus-1/system.d/yggd.conf
```

Then start `yggd`, ensuring the environment variable `DBUS_SESSION_BUS_ADDRESS`
is undefined.

```
sudo go run ./cmd/yggd --server tcp://test.mosquitto.org:8883 --log-level trace --client-id $(hostname)
```

# Running `yggd`

`yggd` can be compiled using meson, or can be run directly with the `go run`
command. It can read configuration values from a file by running
`yggd` with the `--config` option. A sample configuration file is included in
the `data/yggdrasil` directory.

```
sudo go run ./cmd/yggd --config ./data/yggdrasil/config.toml
```

Many default paths (such as Prefix, BinDir, LocalstateDir, etc), as well as some
other compile-time constants, can be specified by providing a linker `-X` flag
argument. See the `Makefile` or `constants.go` for a complete list. 

## Debugging `yggd`

`yggd` can be run within the Delve debugger to make development easier. Install
`dlv` in the guest if it is not already installed:

```
sudo go install github.com/go-delve/delve/cmd/dlv@latest
```

You may need to open TCP port 2345 on the guest. For example, to open the
port using firewalld, run:

```
sudo firewall-cmd --zone public --add-port 2345/tcp --permanent
```

Start `dlv` using the `debug` command:

```
sudo /root/go/bin/dlv debug --api-version 2 --headless --listen 0.0.0.0:2345 ./cmd/yggd -- --config ./data/yggdrasil/config.toml
```

Next, from your host, connect to the dlv server, using either `dlv attach` or by
creating a launch configuration in Visual Studio Code:

```json
{
    "name": "Connect to server",
    "type": "go",
    "request": "attach",
    "mode": "remote",
    "remotePath": "${workspaceFolder}",
    "port": 2345,
    "host": "192.168.122.98"
}
```

Make sure to replace "host" with your virtual machine's IP address.

# Useful Utilities

## `yggctl`

`yggctl` is a program that can interact with a running `yggd` process over an
RPC interface. It is currently very limited in its functionality. Until this
program provides usefulness to users, rather than just to developers, it will
not be installed by default. It can be run directly with `go run`, or installed
with `go install`.

```
go install ./cmd/yggctl
```

See the output of `yggctl --help` for available commands.


## `worker/echo`

`echo` is a very simple reference implementation of a worker written in Go.

If you ran `yggd` on a private session bus, you must run the `echo` worker on
the same bus by specifying the `DBUS_SESSION_BUS_ADDRESS` environment variable:

```
sudo DBUS_SESSION_BUS_ADDRESS=unix:abstract=yggd_demo go run ./worker/echo
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
sub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/data/in -topic yggdrasil/$CLIENT_ID/data/out -topic yggdrasil/$CLIENT_ID/control/out
```

## Publish a message

A client can be sent a `PING` command by generated a control message and
publishing it to the client's "control/in" topic:

```
yggctl generate control-message --type command '{"command":"ping"}' | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/control/in
```

Activity should occur on the terminal attached to `yggd`, and a `PONG` event
message should be received on the "control/out" topic, subscribed to in
**Monitoring topics**.

Similarly, a data message can be published to a client using `yggctl generate`
and `pub`.

```
yggctl generate data-message --directive echo hello | pub -broker tcp://test.mosquitto.org:8883 -topic yggdrasil/$CLIENT_ID/data/in
```

# Code Guidelines

* Commits follow the [Conventional Commits](https://www.conventionalcommits.org)
  pattern.
* Commit messages should include a concise subject line that completes the
  following phrase: "when applied, this commit will...". The body of the commit
  should further expand on this statement with additional relevant details.
* Communicate errors through return values, not logging. Library functions in
  particular should follow this guideline. You never know under which condition
  a library function will be called, so excessive logging should be avoided.
* Code useful to `cmd/*` packages or external third-party packages should exist
  in the top-level package.
* Code useful to `cmd/*` packages, but not external packages should exist in the
  top-level `internal` package.
* Code should exist in a package only if it can be useful when imported
  exclusively.
* Code can exist in a package if it provides an alternative interface to
  another package, and the two packages cannot be imported together.

## Required Reading

* [Effective Go](https://go.dev/doc/effective_go)
* [CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)
* [Go Proverbs](https://go-proverbs.github.io/)

In addition to these 3 "classics", [A collection of Go style
guides](https://golangexample.com/a-collection-of-go-style-guides/) contains a
wealth of resources on writing idiomatic Go.

# Contact

Chat on Matrix: [#yggd:matrix.org](https://matrix.to/#/#yggd:matrix.org).
