This directory includes files necessary to build an SRPM as part of the normal
`meson compile` step.

# Usage

1. Set the `build_srpm` build option to `True` when setting up the project:
   `meson setup -Dbuild_srpm=True builddir`
2. Compile the `srpm` target: `meson compile srpm -C builddir`

An SRPM will be built and can be found in `builddir/dist/srpm`. This SRPM will
need to be rebuilt into a binary in order to install it. To rebuild it using
`mock` on Fedora, for example, run:

```
mock --rebuild ./builddir/dist/srpm/*.src.rpm
```

The RPM built by `mock` can be found (typically) in
`/var/run/mock/<root-name>/results`.
