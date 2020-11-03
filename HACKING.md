# Prerequisites

* _MQTT broker_: `mosquitto` is extremely easy to set up.
* _HTTP server_

# Run `./cmd/ygg`

`go run ./cmd/ygg register --username $(whoami) --password sw0rdf1sh`

# Run `./cmd/yggd`

`go run ./cmd/yggd`

# Call Graphs

Call graphs can be generated to provide a high-level overview of the interactions
between packages.

For basic call graphs, install `go-callvis` (`go get -u github.com/ofabry/go-callvis`) and run:

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

* Communicate errors through return values, not logging. Library functions in
  particular should follow this guideline. You never know under which condition
  a library function will be called, so excessive logging should be avoided.
