[![PkgGoDev](https://pkg.go.dev/badge/github.com/redhatinsights/yggdrasil)](https://pkg.go.dev/github.com/redhatinsights/yggdrasil)

# yggdrasil

yggdrasil is pair of utilities that register systems with RHSM and establishes
a receiving queue for instructions to be sent to the system via a broker.

## Code Architecture

### `./cmd/ygg`

`ygg` is a specialized RHSM client. When run with the `register` subcommand, it
attempts to register with RHSM over D-Bus. If registration is successful, it
activates the `yggd` systemd service.

### `./cmd/yggd`

`yggd` is a systemd service that subscribes to an MQTT topic and dispatches
messages to an appropriate handler.

## Configuration

Configuration of `yggd` and `ygg` can be done by specifying values in a
configuration file or via command-line arguments. Command-line arguments take
precendence over configuration file values. The configuration file is
[TOML](https:/toml.io).

The system-wide configuration file is located at `/etc/yggdrasil/config.toml`.
The location of the file may be overridden by passing the `--config` command-
line argument to `yggd` or `ygg`.

## Message Handling

`yggd` is the program responsible for handling MQTT messages. When a new message
arrives, it unmarshals the message into a `WorkAssignment` message.

```protobuf
// A WorkAssignment message contains the URL locations to retrieve a work
// payload and return the results.
message WorkAssignment {
    // The timestamp when the message was sent.
    string sent = 1;

    // The type of work that is assigned.
    string type = 2;

    // A URL that contains a Work message.
    string payload_url = 3;

    // A URL to POST the completed Work message.
    string return_url = 4;
}
```

It then opens an HTTP request to the URL specified in `payload_url`. It
unmarshals the HTTP response body into a `Work` message.

```protobuf
// A Work message contains information about a work assignment.
message Work {
    // The work assignment identifier.
    string id = 1;

    // The work assignment payload.
    repeated bytes data = 2;
}
```

Then if a worker has registered itself as a handler of the `WorkAssignment`
`type`, `yggd` passes off the `Work` message to the worker by calling the `Start`
gRPC method.

A worker can either accept or reject the work. If it accepts the work, it is
expected to perform the work as defined by its domain, "type" and instructions
in the `data` field. When a worker has completed the work, it must call back to
`yggd` by calling the `Finish` gRPC method on the dispatcher. It passes back a
new `Work` message.

Upon receipt of a finished unit of work, `yggd` opens a new HTTP request to the
URL specified in the original `WorkAssignment` message's `return_url` field. It
uploads the `Work` message it received from the worker as the request body.
