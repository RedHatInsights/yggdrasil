# Prerequisites

In order to run `./cmd/yggd` on the system bus as your local user, you will need
to create the following files:

```
cp ./data/dbus/yggdrasil.conf /etc/dbus-1/system.d/yggdrasil.conf
```

Add the user policy permitting your user (`whoami`) to own `com.redhat.yggddrasil`
on the system bus:

```xml
  <policy user="yggdrasil">
    <allow own="com.redhat.yggdrasil1"/>
  </policy>
```

# Run `./cmd/ygg-exec`

`go run ./cmd/ygg-exec --base-url https://cloud.stage.redhat.com/api --auth-mode basic --username ${USER} --password s3cr3t upload --collector advisor $HOME/insights-ic-rhel8-dev-thelio-20200521100458.tar.gz`

# Run `./cmd/yggd`

`go run ./cmd/yggd --base-url https://cloud.stage.redhat.com/api --auth-mode basic --username ${USER} --password s3cr3t --interface-file ${PWD}/data/dbus/com.redhat.yggdrasil.xml`

## GDBus

You can install D-Feet to browse the bus objects in a graphical way, or use
`gdbus` to send methods directly.

```bash
gdbus introspect --system \
    --dest com.redhat.yggdrasil1 \
    --object-path /com/redhat/yggdrasil
gdbus call --system \
    --dest com.redhat.yggdrasil1 \
    --object-path /com/redhat/yggdrasil \
    --method com.redhat.yggdrasil1.Upload \
    "$HOME/insights-ic-rhel8-dev-thelio-20200521100458.tar.gz" "advisor" "{}"
```

# Call Graphs

Call graphs can be generated to provide a high-level overview of the interactions
between packages.

For basic call graphs, install `go-callvis` (`go get -u github.com/ofabry/go-callvis`) and run:

```bash
# Call graph of the main function of yggd, up to calls into the yggdrasil package
go-callvis -nostd -format png -file yggdrasil.main ./cmd/yggd
# Call graph of the yggdrasil package, as invoked by yggd
go-callvis -nostd -format png -file yggdrasil.yggdrasil -focus github.com/redhatinsights/yggdrasil/pkg ./cmd/yggd
# Call graph of the main function of ygg-exec, up to calls into the yggdrasil package
go-callvis -nostd -format png -file ygg-exec.main ./cmd/ygg-exec
# Call graph of the yggdrasil package, as invoked by ygg-exec
go-callvis -nostd -format png -file ygg-exec.yggdrasil -focus github.com/redhatinsights/yggdrasil/pkg ./cmd/ygg-exec
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
callgraph -algo pta -format digraph ./cmd/ygg-exec | grep github.com/redhatinsights/yggdrasil | sort | uniq | digraph
```
