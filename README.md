[![PkgGoDev](https://pkg.go.dev/badge/github.com/redhatinsights/yggdrasil)](https://pkg.go.dev/github.com/redhatinsights/yggdrasil)

# yggdrasil

yggdrasil is a collection of utilities and system services that provide other
system services a single interface to interact with Red Hat Platform services.

yggdrasil provides functionality that fall into 3 categories.

* Scheduling and running collections
  * This includes an optional redaction process over which the data of a
    collection is subjected prior to being archived and prepared for upload.
* Uploading archives or arbitrary payloads with a valid Content-Type
* Downloading collection modules

## Collection Modules

Collection modules are file archives that contain all the code and data
necessary to run a collection. They can be enabled or disabled interactively by
a system administrator via the D-Bus API, or statically by editing the yggdrasil
configuration file (/etc/yggdrasil/yggdrasil.conf).

A collection module contains:

* config.ini: A config file that contains ancillary data about the collection
  module (see sample below).
* collect: An executable file that is the entrypoint to collection. This file is
  executed by `yggd` when a collection is initiated.

A collection may contain any additional files or directories that will be made
available to the `collect` program at runtime.

### Sample config.ini

```
[Collection]
Name=foo
AutoUpdateURL=http://cloud.foo/bar/var/lib/foo.egg
Frequency=24h
```

### Running a Collection

`yggd` will invoke a collection module's `/collect` entrypoint, passing in a
JSON object to its `stdin`. This JSON object defines parameters under which the
collection module is expected to operate. Examples include the destination path
to write collected data or files. A collection object must adhere to the
parameters specified in the JSON object; failure to do so will result in a
failed collection attempt.

#### Sample JSON input

```json
{
    "v1": {
        "output_dir": "/tmp/yggd.I1nyqpcgeX"
    }
}
```

### D-Bus Interface

Most methods in the package library (see below) will map directly to a D-Bus
method. This design is intentional; it makes the interactions between a client,
a D-Bus server object, and the base library straightforward. For example, an
`Upload` function defined in the package library will have a corresponding
`Upload` D-Bus method, exported on the `com.redhat.yggdrasil1` D-Bus interface.

## Code Architecture

### `./` - Go package implementing functional-level behavior

This package implements the bulk of the yggdrasil functionality. While it is a
public package that downstream Go projects can consume, its primary purpose is
to provide a testable interface to `yggd` and `ygg-exec`. For example, one could
implement a custom uploader using the `yggdrasil.Upload()` functions. However,
it is recommended to interact with `yggd` through the D-Bus interface or
directly via `ygg-exec`.

### `cmd/yggd` - System daemon

This package is a program (`yggd`) that is intended to be run on a host as a
system daemon. It implements a D-Bus interface and exports an object onto the
system D-Bus for clients to interact with. `yggd` is the main consumer of the 
`yggdrasil` package implemented under `pkg/` and is the primary service with
which clients should be interacting.

### `internal/` - Go package implementing functionality unique to yggdrasil

This package contains structures and functions that enable `cmd/yggd` and/or
`cmd/ygg-exec`, but aren't necessarily useful to a consumer at the package
level. For example, this package contains the XML interface files as well as the
source code files that `yggd` uses to implement interfaces and export objects
onto the system D-Bus.

### `cmd/ygg-exec` - oneshot entrypoint to perform a single ad-hoc operation

This utility provides a CLI to perform a single operation immediately, in
process. It is important to note that this utility *does not* interact with
`yggd`. This makes it particularly well suited for containerized environments
where  running a full system daemon is not feasible. All `yggdrasil` operations
are executed in process as if they were executed by a `yggd` process.

For example, to upload a pre-existing archive:

`ygg-exec upload --content-type foo /var/tmp/foo.tar.gz`

## Configuration

Configuration of `yggd` and `ygg-exec` can be done by specifying values in a
configuration file or via command-line arguments. Command-line arguments take
precendence over configuration file values. The configuration file is
[TOML|https:/toml.io].

When running `yggd` or `ygg-exec` as root, a system-wide configuration file is
loaded, located at `/etc/yggdrasil/config.toml`. If `ygg-exec` or `yggd` is run
as a non-root user, a configuration file located inside the user's home
directory is loaded: `~/.config/yggdrasil/config.toml`. Note that either the
system-wide file or the local home directory file is loaded, but not both. If
the default configuration file does not exist, it is not created. Instead, only
command-line argument values are used.

The location of the file may be overridden by passing the `--config` command-
line argument to `yggd` or `ygg-exec`.
