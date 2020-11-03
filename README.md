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
[TOML|https:/toml.io].

The system-wide configuration file is located at `/etc/yggdrasil/config.toml`.
The location of the file may be overridden by passing the `--config` command-
line argument to `yggd` or `ygg`.
