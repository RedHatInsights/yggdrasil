# Prerequisites

* _MQTT broker_: `mosquitto` is extremely easy to set up.
* _HTTP server_
* Development package dependencies are listed as `BuildRequires:` in the
  `yggdrasil.spec.in` file.
  * NOTE: These are the packages names are listed as they exist in [Fedora
    Linux](https://getfedora.org/), [CentOS Stream](https://centos.org/), and
    [Red Hat Enterprise
    Linux](https://www.redhat.com/en/technologies/linux-platforms/enterprise-linux).
    Similar package names for other distros may vary.

# Getting Started

Compile and install programs and data:

```
make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var
sudo make PREFIX=/usr SYSCONFDIR=/etc LOCALSTATEDIR=/var install
```

If the system is not yet registered with RHSM, running `ygg connect` will
register the system with RHSM and activate the `yggd` systemd service.

```
sudo ygg connect --username $(whoami) --password sw0rdf1sh
```

Disable the systemd service in order to run `yggd` locally.

```
sudo systemctl disable --now yggd
```

Run `yggd`, connecting it to the broker running on localhost.

```
sudo go run ./cmd/yggd --log-level trace --broker tcp://127.0.0.1:1883 --cert-file /etc/pki/consumer/cert.pem --key-file /etc/pki/consumer/key.pem
```

Only the `echo` worker is shipped as part of the yggdrasil software
distribution, but it is not compiled or installed. It must be built and
installed manually:

```
go build ./worker/echo
sudo install -D -m 755 echo /usr/libexec/yggdrasil/echo-worker
```

A running instance of `yggd` should detect the worker and execute it.

With a running `yggd` process, data messages can be sent to it over the
appropriate MQTT topic.

Send `yggd` a "ping" command:

```
export CONSUMER_ID=$(openssl x509 -in cert.pem -subject -nocert | cut -f3 -d" ")
mosquitto_pub --host 127.0.0.1 --port 1883 --topic "yggdrasil/${CONSUMER_ID}/control/in" --message "{\"type\":\"command\",\"message_id\":\"$(uuidgen | tr -d '\n')\",\"version\":1,\"sent\":\"$(date --iso-8601=seconds --utc | tr -d '\n')\",\"content\":{\"command\":\"ping\"}}"
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
# Call graph of the main function of ygg, up to calls into the yggdrasil package
go-callvis -nostd -format png -file ygg.main ./cmd/ygg
# Call graph of the yggdrasil package, as invoked by ygg
go-callvis -nostd -format png -file ygg.yggdrasil -focus github.com/redhatinsights/yggdrasil ./cmd/ygg
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
callgraph -algo pta -format digraph ./cmd/ygg | grep github.com/redhatinsights/yggdrasil | sort | uniq | digraph
```

# Code Guidelines

* Commits follow the [Conventional Commits](https://www.conventionalcommits.org)
  pattern.
* Communicate errors through return values, not logging. Library functions in
  particular should follow this guideline. You never know under which condition
  a library function will be called, so excessive logging should be avoided.

# Contact Us

Chat on IRC: #yggd on [Libera](https://libera.chat).
