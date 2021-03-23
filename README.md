[![PkgGoDev](https://pkg.go.dev/badge/github.com/redhatinsights/yggdrasil)](https://pkg.go.dev/github.com/redhatinsights/yggdrasil)

# yggdrasil

yggdrasil is pair of utilities that register systems with RHSM and establishes
a receiving queue for instructions to be sent to the system via a broker.

## Code Architecture

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
caller should then pass that channel into a handler function that receives
values from the channel and handles them appropriately.

```
var messageRouter MessageRouter
var dispatcher Dispatcher

// Connect returns a newly created channel that will receive values when the
// messageRouter emits the SignalMessageRecv signal.
c := messageRouter.Connect(yggdrasil.SignalMessageRecv)

// Pass the channel into a handler function that runs in a goroutine.
go dispatcher.HandleMessageRecvSignal(c)
```

Types all expect to share state through a common `*memdb.MemDB`. An
appropriately configured `*memdb.MemDB` can be received by calling
`yggdrasil.NewDatastore()`. This value needs to be passed to each type during
creation. It is through this shared database that types can access values. Most
signals emit a message ID that can be used to retrieve a message from the
database.

For details on the types and their available API, see [the
GoDocs](https://pkg.go.dev/github.com/redhatinsights/yggdrasil).

#### `MessageRouter`

[`MessageRouter`](https://pkg.go.dev/github.com/redhatinsights/yggdrasil#MessageRouter)
is a customized MQTT client. It implements the protocol defined in
[cloud-connector](https://github.com/RedHatInsights/cloud-connector#protocol).
After creating a `MessageRouter`, the router must be connected to an MQTT broker
by calling the `ConnectClient` method. The `SubscribeAndRoute` method will start
two subscription routines; one for the inbound control topic, and one for the
inbound data topic (see [the protocol specification for
details](https://github.com/RedHatInsights/cloud-connector#protocol)). The MQTT
message handler function for both topics will unmarshal payloads received in
MQTT messages. When a message is receieved on the data topic, it is unmarshaled
into a `Data` struct, stored in the `MemDB`, and the `MessageRouter` emits the
message ID on the "data-recv" signal. When a message is received on the inbound
control topic, it is handled appropriately. For example, a "ping" command
generates a "pong" `Event` that is published on the outbound control topic.

A `MessageRouter` can announce itself by calling the `PublishConnectionStatus`
method. This method collects "canonical facts" about the system its running on
and publishes a
[`ConnectionStatus`](https://pkg.go.dev/github.com/redhatinsights/yggdrasil#ConnectionStatus)
message on the outbound control topic.

There is a convenience method, `ConnectPublishSubscribeAndRoute`, that does all
of the above in the "correct" order.

#### `DataProcessor`

[`DataProcessor`](https://pkg.go.dev/github.com/redhatinsights/yggdrasil#DataProcessor)
is a data transformer. It has no directly callable methods. Instead, it handles
"data-recv" signals from a `MessageRouter` and "data-return" signals from a
`Dispatcher`.

##### `HandleDataRecvSignal`

This handler receives message IDs from a `MessageRouter` when a `Data` message
is emitted on the "data-recv" signal. It fetches the `Data` message from the
database and looks up the worker it is destined for. If the worker has been
registered with `detachedContent` set to `false`, the `DataProcessor` proceeds
with emitting the message ID on the "data-process" signal. If `detachedContent`
is `true`, then the `DataProcessor` creates an HTTP GET request. It parses the
value of the `Data` message's `Content` field as a URL, and sends a GET request
to the URL. If the URL response is 200, any data in the response body is written
into the `Data` message's `Content` field, **replacing** the original value. The
`DataProcessor` then proceeds to emit the message ID on the "data-process"
signal.

##### `HandleDataReturnSignal`

This handler receives message IDs from a `Dispatcher` when a `Data` message is
emitted on the "data-return" signal. It fetches the `Data` message from the
database along with the original message the `Data` message is in response to
(by looking for a message with the message ID specified in the `Data` message's
`ResponseTo` field). Finally, a worker matching the **original** `Data`
message's `Directive` field is fetched from the database. If the worker has been
registered with `detachedContent` set to `true`, an HTTP POST request is
created. The URL of the request is set to the value of the received `Data`
message's `Directive` field. The request body is set to the value of the
received `Data` message's `Content` field. Any key/value pairs found in the
received `Data` message's `Metadata` field are added as headers to the HTTP
request. The HTTP request is then sent by the client and the received `Data`
message's ID is emitted on the "data-consume" signal.

#### `Dispatcher`

[`Dispatcher`](https://pkg.go.dev/github.com/redhatinsights/yggdrasil#Dispatcher)
implements the gRPC "Dispatcher" service, as defined in the
[protocol](https://github.com/RedHatInsights/yggdrasil/blob/main/protocol/yggdrasil.proto).
After creating a `Dispatcher`, it must be started by calling the
`ListenAndServe` method. This method will start listening on a UNIX domain
socket and serve the gRPC "Dispatcher" methods on the socket. It expects that
worker processes needing to communicte with it to call in via the service's two
methods: "Register" and "Send".

##### gRPC "Register"

A worker must register itself with the `Dispatcher` by calling the `Register`
method, passing an appropriately constructed `RegistrationRequest` message. A
`RegistrationRequest` message consists of the following fields:

| **Field**         | **Description** |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Handler`         | A unique string identifying the worker in the system. This value is referenced by external messages as the `Directive`.                                                                            |
| `Pid`             | The worker's process ID.                                                                                                                                                                           |
| `DetachedContent` | Setting this to true will reroute the source of the `Content` field to a URL instead of embedded in the message. Likewise, response `Data` messages have their data sent via HTTP instead of MQTT. |
| `Features`        | A map of string key/value pairs that are sent with the `MessageRouter`'s published `ConnectionStatus` message.                                                                                     |

When a worker successfully registers itself by calling this method, it receives
a socket address as part of the response. The worker is expected to be listening
on this address and serving the "Worker" service (as defined by the gRPC
protocol).

##### gRPC "Send"

A worker may call this method to send a `Data` message. The implementation of
this method looks up the originating message with the value of the `Data`
message's "MessageID" field. It then creates a new `Data` message, setting the
value of the `ResponseTo` field to the original message ID. The new message is
inserted into the database, and its message ID is emitted on the "data-return"
signal.

##### `HandleDataProcessSignal`

This handler receives message ID values from a `DataProcessor` emitted on the
"data-process" signal. It looks up the message in the database, looks up the
worker the message is destined for, dials the worker over gRPC, creates a
protobuf `Data` message and calls the "Send" RPC method on the worker. It then
emits the message ID on the "data-dispatch" signal.

#### `ProcessManager`

[`ProcessManager`](https://pkg.go.dev/github.com/redhatinsights/yggdrasil#ProcessManager)
is a specialized process orchestrator. Once created, it can be operated by
starting worker processes explicitly via `StartProcess`, or all at once via
`BootstrapWorkers`. Additionally, workers can be started via inotify events by
calling `WatchForProcesses`.

### `./cmd/ygg`

`ygg` is a specialized RHSM client. When run with the `connect` subcommand, it
attempts to register with RHSM over D-Bus. If registration is successful, it
activates the `yggd` systemd service.

### `./cmd/yggd`

`yggd` is a systemd service that subscribes to an MQTT topic and dispatches
messages to an appropriate handler. It creates a `MessageRouter`, `Dispatcher`,
`DataProcessor` and `ProcessManager` and connects them in the following pattern.

| **From**        | **Signal**               | **To**           |
| --------------- | ------------------------ | ---------------- |
| `dispatcher`    | `SignalProcessDie`       | `processManager` |
| `messageRouter` | `SignalWorkerUnregister` | `dispatcher`     |
| `messageRouter` | `SignalWorkerRegister`   | `dispatcher`     |
| `dataProcessor` | `SignalDataRecv`         | `messageRouter`  |
| `dispatcher`    | `SignalDataProcess`      | `dataProcessor`  |
| `dataProcessor` | `SignalDataReturn`       | `dispatcher`     |
| `messageRouter` | `SignalDataConsume`      | `dataProcessor`  |

### Messages

"Messages" are the main type of data passed between the various data structures
in the package. Messages come in two forms: Commands and Data.

#### Commands

The "control" topic is a topic over which the client receives `Command`
messages. These messages instruct the client to perform a specific operation.
Currently supported command values as of this writing are: "ping",
"disconnect" and "reconnect".

#### Data

The "data" topic is a topic over which the client receives `Data` messages.
These messages include a "directive", instructing the client to route the data
to a specific worker process.

## Configuration

Configuration of `yggd` and `ygg` can be done by specifying values in a
configuration file or via command-line arguments. Command-line arguments take
precendence over configuration file values. The configuration file is
[TOML](https:/toml.io).

The system-wide configuration file is located at `/etc/yggdrasil/config.toml`.
The location of the file may be overridden by passing the `--config` command-
line argument to `yggd`.

## Workers

`yggd` through the `Dispatcher` has a dispatcher/worker protocol defined in
`protocol/yggdrasil.proto`. This protocol defines two services (`Dispatcher` and
`Worker`), and the messages necessary for the services to exchange data. In
order to interact with `yggd` as a worker process an executable must be
installed into `$LIBEXECDIR/yggdrasil/`. A pkg-config module named 'yggdrasil'
is installed so that workers can locate the worker exec directory at build time.

```
pkg-config --variable workerexecdir yggdrasil
/usr/local/libexec/yggdrasil
```

The executable name must end with the string "worker". `yggd` will execute
worker programs at start up. It will set the `YGG_SOCKET_ADDR` variable in the
worker's environment. This address is the socket on which the worker must dial
the dispatcher and call the "Register" RPC method. Upon successful registration,
the worker will receive back a socket address. The worker must bind to and
listen on this address for RPC methods. See `worker/echo` for an example worker
process that does nothing more than return the content data it received from the
dispatcher.
