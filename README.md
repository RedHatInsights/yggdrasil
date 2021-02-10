
















[![PkgGoDev](https://pkg.go.dev/badge/github.com/redhatinsights/yggdrasil)](https://pkg.go.dev/github.com/redhatinsights/yggdrasil)

# yggdrasil

yggdrasil is pair of utilities that register systems with RHSM and establishes
a receiving queue for instructions to be sent to the system via a broker.

## Code Architecture

### `./cmd/ygg`

`ygg` is a specialized RHSM client. When run with the `register` subcommand, it
attempts to register with RHSM over D-Bus. If registration is successful, it
activates the `yggd` systemd service.

### `package yggdrasil`

`package yggdrasil` is a Go package that provides an API suitable to create a
yggdrasil service daemon. The program in `./cmd/yggd` is the canonical reference
daemon; however the design goal of this package is to enable more customized
service daemons to be written.

`yggdrasil` consists of a handful of types that can be connected together to
construct a service. The types are:

*  `MessageRouter`
*  `DataProcessor`
*  `Dispatcher`
*  `ProcessManager`

All types operate independently from one another. Communication between types
happens through a signalling concept. A type "emits" a signal and sends a value
on a set of channels associated with the signal. Any other type that has
connected to the emitted signal will be able to receive values on the channel
when the signal is "emitted". For example, to connect a `dispatcher` to a
`messageRouter`'s `SignalMessageRecv` signal, one calls the `Connect` method on
the `messageRouter`. This method returns a unique channel to the caller. The
caller should then pass that channel into a handler function that can itertively
receive values from the channel and handle them appropriately.

```
var messageRouter MessageRouter
var dispatcher Dispatcher

// Connect returns a newly created channel that will receive values when the
// messageRouter emits the SignalMessageRecv signal.
c := messageRouter.Connect(yggdrasil.SignalMessageRecv)

// Pass the channel into a handler function that runs in a goroutine.
go dispatcher.HandleMessageRecvSignal(c)
```

Types all expect to share state through a common `*memdb.MemDB`. An appropriately
configured `*memdb.MemDB` can be received by calling `yggdrasil.NewDatastore()`.
This value needs to be passed to each type during creation. It is through this
shared database that types can access values. The signals emit a message ID that
can be used to retrieve a message from the database.

For details on the types and their available API, see [the GoDocs](https://pkg.go.dev/github.com/redhatinsights/yggdrasil).

### Messages

`MessageRouter` is a specialized MQTT client that can be used to send and
receive JSON objects over a pair of specialized topics: a "control" topic and
a "data" topic.

#### Commands

The "control" topic is a topic over which the client receives `Command`
messages. These messages instruct the client to perform a specific operation.
Currently supported command values as of this writing are: "ping",
"disconnect" and "reconnect".

#### Data

The "data" topic is a topic over which the client receives `Data` messages.
These messages include a "directive", instructing the client to route the data
to a specific worker process.

### `./cmd/yggd`

`yggd` is a systemd service that subscribes to an MQTT topic and dispatches
messages to an appropriate handler. It creates a `MessageRouter`, `Dispatcher`,
`DataProcessor` and `ProcessManager` and connects them in the following pattern.

* `messageRouter` --> "data-recv" --> `dataProcessor`
* `dataProcessor` --> "data-process" --> `dispatcher`
* `dispatcher` --> "data-return" --> `dataProcessor`
* `dataProcessor` --> "data-consume" --> `messageRouter`

`messageRouter` connects to the broker, publishes a `ConnectionStaus` message.
It then subscribes to the control and data topics.

`dataProcessor` connects to the `messageRouter`'s `SignalDataRecv` signal. In
its signal handler function, it receives the message ID from the channel, looks
up the message by ID and a worker registered with the handler specified in the
"directive" field. If the worker has the `remotePayload` value set to `true`,
the `dataProcessor` assumes the payload of the `Data` message contains a URL
and creates an HTTP GET request and sends it to that URL. The body of the HTTP
response is then written into the `Data` message "content" field, overwriting
the original "content" value. In either case (`remotePayload` true or false),
`dataProcessor` then emits a `SignalDataProcess` signal.

`dispatcher` connects to the `dataProcessor`'s `SignalDataProcess` signal. In
its signal handler, it receives the message ID from the channel, looks up the
message by ID and a worker registered with the handler specified in the
"directive" field. It then attempts to dial the worker over its socket address,
and invokes the gRPC "Send" method. The payload of the `Data` message is sent
as the data to the worker. It then emits a `SignalDataDispatch` signal.

In `dispatcher`'s implementation of the `Dispatcher` service "Send" RPC method,
it receives protobuf `Data` messages from workers. It then creates a *new* `Data` message, sets the "response_to" field to the original `Data` message ID, and loads the data
received from the worker into the new message's "content" field. It then emits
a `SignalDataReturn` signal.

`dataProcessor` connects to the `dispatcher`'s `SignalDataReturn` signal. In its
signal handler, it receives the message ID from the channel and looks up the
message by ID and the worker by "directive". If the worker has `remotePayload`
set to true, `dataProcessor` creates an HTTP POST request, sets the request
body to the value of the "content" field, and sends the request to the URL
specified in the new message's "directive" field. Either way, it then emits a
`SignalDataConsume` signal.

`messageRouter` connects to the `dataProcessor`'s `SignalDataConsume` signal. It
receives the message ID from the channel and looks up the message by ID and the
worker by "directive". If the worker has `remotePayload` set to false, it will
publish the `Data` message on the data topic. Either way, the original and the
new `Data` messages are then deleted from the database.

## Configuration

Configuration of `yggd` and `ygg` can be done by specifying values in a
configuration file or via command-line arguments. Command-line arguments take
precendence over configuration file values. The configuration file is
[TOML](https:/toml.io).

The system-wide configuration file is located at `/etc/yggdrasil/config.toml`.
The location of the file may be overridden by passing the `--config` command-
line argument to `yggd` or `ygg`.

## Workers

`yggd` through the `Dispatcher` has a dispatcher/worker protocol defined in
`protocol/yggdrasil.proto`. This protocol defines two services (`Dispatcher` and
`Worker`), and the messages necessary for the services to exchange data. In
order to interact with `yggd` as a worker process an executable must be
installed into `$LIBEXECDIR/yggdrasil/`. The executable name must end with the
string "worker". `yggd` will execute worker programs at start up. It will set
the `YGG_SOCKET_ADDR` variable in the worker's environment. This address is the
socket on which the worker must dial the dispatcher and call the "Register" RPC
method. Upon successful registration, the worker will receive back a socket
address. The worker must bind to and listen on this address for RPC methods.
See `worker/echo` for an example worker process that does nothing more than
return the payload data it received from the dispatcher.
