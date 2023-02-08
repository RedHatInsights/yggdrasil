# yggdrasil "echo" worker

This "echo" worker is a simple yggdrasil worker that echoes back the data it is
given. It's main function is to provide an example and reference implementation
for how a worker could be developed.

# Running

The worker can run directly without needing to install it first (`go run .`).
By default, the worker will attempt to connect to an appropriate D-Bus (session
or system), depending on whether `DBUS_STARTER_BUS_TYPE` is set to "session".

# D-Bus Service Activation

The worker can be started automatically by the broker. Install the included file
`com.redhat.yggdrasil.Worker1.echo.service` into
`/usr/share/dbus-1/system-services`. When the broker receives a message for the
address specified by the `Name=` directive, the worker will be started before
the message is delivered.

Should your worker require more advanced start up functionality, it is possible
to specify a value for the `SystemdService=` directive. With such a directive in
the service activation file, the D-Bus broker will ask systemd to start the
named service. Within that system service file, you can define any resource
limitations or environment variables needed for your worker. For example:

```
[Unit]
Description=yggdrasil echo worker

[Service]
Type=dbus
BusName=com.redhat.yggdrasil.Worker1.echo
ExecStart=/usr/libexec/yggdrasil/echo
```

Note: There is no `[Install]` section; this is because the service is D-Bus
activatable.

See the [systemd service
examples](https://www.freedesktop.org/software/systemd/man/systemd.service.html#Examples)
for detail.
