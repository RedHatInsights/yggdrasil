# AGENTS.md

This file contains guidance for coding agents when helping with development in
this repository.

## What is included in this repository

This repository contains source code for two binaries: `yggd`, which runs as a
system daemon, and `yggctl`, which is a CLI tool for controlling a running
`yggd` instance. It also includes an example worker, [worker/echo](worker/echo),
showing how to implement the `com.redhat.Yggdrasil1.Worker1` interface.

_yggd_ subscribes to topics on an MQTT broker and routes data to child worker
processes over D-Bus. The dispatcher exposes `com.redhat.Yggdrasil1.Dispatcher1`.

Release lines in this repository: **`main`** (0.4.x, RHEL 10+) and maintenance branches
such as **`yggdrasil-0.2.x`** (RHEL 9 era). Distro package names do not always
match the branch — on RHEL 10+, the daemon ships in the **`yggdrasil`** RPM
(0.4.x); on RHEL 9, the equivalent **`rhcd`** service was provided through the
**`rhc`** RPM (0.2.x source line). The [rhc](https://github.com/RedHatInsights/rhc)
project is a separate repository — yggdrasil source code has never been part of
that repo. `rhc` still activates `yggd`/`rhcd` when connecting a host to Red Hat
services.

| Path | Purpose |
|------|---------|
| `cmd/yggd` | Main system daemon |
| `cmd/yggctl` | Control CLI for `yggd` (always installed to `bindir` by meson/RPM, e.g. `/usr/bin/yggctl`) |
| `internal/` | Private packages (`config`, `transport`, `work`, `messagejournal`, …) |
| `ipc/` | D-Bus interface definitions (`Dispatcher1`, `Worker1`) |
| `worker/` | Worker SDK; see [worker/echo](worker/echo) for the reference worker (installed only when meson `-Dexamples=true`; enabled in RPM builds) |
| `dbus/` | D-Bus policy and systemd unit templates |
| `data/` | Sample config (`config.toml`), tags, facts files |
| `integration-tests/` | Python/pytest integration tests (installed RPM + MQTT + D-Bus) |
| `systemtest/` | TMT plan for Testing Farm / Packit system tests |
| `dist/srpm/` | SRPM packaging via meson |
| `doc/` | Architecture notes and sequence diagrams |

Top-level public API: `messages.go`, `util.go`. See [doc/yggd.md](doc/yggd.md) for
the `transport.Transporter` / `main.Client` / `work.Dispatcher` data flow.

## Building

Building is described in [README.md](README.md). Local development uses Go;
packaging and installation use meson.

### Go (development)

Requires Go **1.24+** (see [go.mod](go.mod)):

```bash
go build -v ./...
```

Run `yggd` directly during development:

```bash
go run ./cmd/yggd --server tcp://localhost:1883 --log-level trace --client-id $(hostname)
```

Full local quickstart (mosquitto, echo worker, MQTT pub/sub):
[CONTRIBUTING.md](CONTRIBUTING.md).

### Meson (packaging / install)

Production builds use meson for systemd units, D-Bus policy, and RPM layout.
Requires development packages for D-Bus, systemd, and bash-completion (see
[`meson.build`](meson.build); package names vary by distribution):

```bash
meson setup --prefix /usr/local --sysconfdir /etc --localstatedir /var builddir
meson compile -C builddir
meson install -C builddir
```

SRPM: `meson setup -Dbuild_srpm=True builddir && meson compile srpm -C builddir`
— see [dist/srpm/README.md](dist/srpm/README.md).

Packit/COPR builds use `.packit.yaml` (targets: RHEL 10, CentOS Stream 10,
Fedora).

## Testing

Go unit tests cover library packages under `internal/` and selected top-level
code. End-to-end MQTT and D-Bus behavior is exercised by
`integration-tests/` (Python), not by Go unit tests alone.

### Unit tests (Go)

```bash
go test -v ./...
```

Unit tests run in CI via [`.github/workflows/go.yml`](.github/workflows/go.yml).

### Static analysis

`go vet` runs static analyzers on the source code; it does not execute unit
tests.

```bash
go vet -v ./...
```

`go vet` also runs in [`.github/workflows/go.yml`](.github/workflows/go.yml).

Before submitting changes, mirror the GitHub Actions checks locally:

```bash
go get .
go build -v ./...
go test -v ./...
go vet -v ./...
golangci-lint run --verbose --timeout=3m
go install github.com/segmentio/golines@latest
if [ "$($(go env GOPATH)/bin/golines . --dry-run | wc -l)" -gt 0 ]; then exit 1; fi
```

Install `golangci-lint` locally if needed:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
$(go env GOPATH)/bin/golangci-lint run --verbose --timeout=3m
```

CI also runs **commitsar** (conventional commits), **woke** (inclusive language),
and **Packit** (COPR build + TMT system tests). Lint steps are defined in
[`.github/workflows/lint.yml`](.github/workflows/lint.yml).

### Integration tests (Python)

Integration tests require `yggdrasil` (and workers such as `echo`) **installed
from an RPM** on the test host — they exercise the systemd service, config under
`/etc/yggdrasil/`, and system `yggctl`, not a `go run` build from the source
tree. Build and install via meson, or use a distro/COPR package (see
[systemtest/install-test-deps.sh](systemtest/install-test-deps.sh)).

Also requires mosquitto, D-Bus, and systemd. See [CONTRIBUTING.md](CONTRIBUTING.md)
for additional prerequisites. Start the broker before running tests:

```bash
systemctl start mosquitto
python3 -m venv venv
venv/bin/pip install --upgrade pip
venv/bin/pip install -r integration-tests/requirements.txt
venv/bin/pytest -v integration-tests
```

Or with an activated shell: `source venv/bin/activate && pytest -v integration-tests`

The systemtest plan wraps this in
[systemtest/tests/integration/test.sh](systemtest/tests/integration/test.sh).
Tier markers: `tier1` (see `integration-tests/conftest.py`).

### System tests (TMT / Testing Farm)

In-tree system tests: [systemtest/plans/main.fmf](systemtest/plans/main.fmf).
Triggered on PRs via Packit (`.packit.yaml`, label `unit`) and optionally via
Testing Farm (`.testing-farm.yaml`).

## Security

`yggd` runs as the **`yggdrasil` system user** (not root). It has privileges to
dispatch messages to workers over D-Bus and connects to MQTT brokers. The
`yggdrasil` user is a member of the **`rhsm` group** to read RHSM consumer
certificate and key files under `/etc/pki/consumer/`. When modifying or
generating code:

- Avoid logging secrets (certificates, keys, broker credentials, tokens).
- Validate paths and inputs before file I/O; do not broaden D-Bus policy or
  systemd unit privileges without explicit review.
- mTLS and RHSM consumer certs are sensitive — never commit or echo them in
  tests or logs.
- Workers run as configurable users (`worker_user` meson option); privilege
  boundaries matter for local privilege escalation tests in `integration-tests/`.
- Communicate errors through return values in library code, not ad-hoc logging
  (see [CONTRIBUTING.md](CONTRIBUTING.md#code-guidelines)).

## Code style and architecture

- **Go 1.24**, module `github.com/redhatinsights/yggdrasil`.
- Format with `gofmt`, `goimports`, and
  [`golines`](https://github.com/segmentio/golines) before committing (golines
  is enforced in CI).
- Package placement (from CONTRIBUTING):
  - Code for `cmd/*` or external importers → top-level package.
  - Code for `cmd/*` only → `internal/`.
  - A package should be importable on its own, or provide a mutually exclusive
    alternative interface.
- D-Bus interfaces live in `ipc/`; workers should use the `worker` package SDK
  rather than reimplementing name-claiming and object export. Follow
  [worker/echo](worker/echo) when adding or changing workers.
- Required reading: [Effective Go](https://go.dev/doc/effective_go),
  [CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments).

Sourcery is configured in [`.sourcery.yaml`](.sourcery.yaml) (test loop/conditional
rules disabled).

## Pull requests

- Ensure all CI checks pass (build, unit tests, lint, woke) before requesting
  review.
- Add or update Go unit tests for changed logic; update integration tests when
  behavior visible to MQTT/D-Bus clients changes.
- Update user-facing docs (`README.md`, `CONTRIBUTING.md`, `doc/`) when
  install paths, config keys, or worker contracts change.

## Git commits

Follow [Conventional Commits](https://www.conventionalcommits.org):

- Subject completes: "when applied, this commit will…"
- Subject and body are separated by a blank line.
- Body expands with relevant detail; detail items start with `* `.
- Prefix types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, etc.
  (see [CONTRIBUTING.md](CONTRIBUTING.md#code-guidelines)).

Example:

```text
docs: add AGENTS.md for AI-assisted development

* Summarize build, test, CI, and contribution conventions for coding agents
```
