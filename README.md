[![godocs.io](https://godocs.io/github.com/redhatinsights/yggdrasil?status.svg)](https://godocs.io/github.com/redhatinsights/yggdrasil)

# yggdrasil

_yggdrasil_ is a system daemon that subscribes to topics on an MQTT broker and
routes any data received on the topics to an appropriate child "worker" process,
exchanging data with its worker processes through a D-Bus message broker.

## Installation

The easiest way to compile and install yggdrasil is using `meson`. Because
yggdrasil runs as a privileged system daemon, systemd unit files and D-Bus
policy files must be installed in specific directories in order for the service
to start.

Generally, it is recommended to follow your distribution's packaging guidelines
for compiling Go programs and installing projects using `meson`. What follows is
a generally acceptable set of steps to setup, compile and install yggdrasil
using `meson`.

```bash
# Set up the project according to distribution-specific directory locations
meson setup --prefix /usr/local --sysconfdir /etc --localstatedir /var builddir
# Compile
meson compile -C builddir
# Install
meson install -C builddir
```

`meson` includes an optional `--destdir` to its `install` subcommand to aid in
packaging.

## Configuration

Configuration of `yggd` can be done by specifying values in a configuration file
or via command line arguments. Command-line arguments take precedence over
configuration file values. The configuration file is [TOML](https://toml.io).

The default location searched for the system-wide configuration file is at
`/etc/yggdrasil/config.toml` (assuming SYSCONFDIR=/etc, as the example above).

The default location of the file may be overridden by passing the `--config`
command-line argument to `yggd`.

### (Optional) Authentication

In order to run `yggd` under certain conditions (such as connecting to a broker
that requires mTLS authentication), a valid certificate must first be created
and written to the filesystem.

#### Red Hat Subscription Manager

One way of generating a valid certificate is to first register the system with
an RHSM provider. The simplest way to do this is to create a free [Red Hat
Developer account](https://developers.redhat.com/register). On a Red Hat
Enterprise Linux system, run `subscription-manager register`, using the
developer account username and password.

```
sudo subscription-manager register --username j_developer@company.com --password sw0rdf1sh
```

Once the system is successfully registered with RHSM, `yggd` can be launched,
using the certificate key pair:

```
sudo /usr/sbin/yggd --cert-file /etc/pki/consumer/cert.pem --key-file /etc/pki/consumer/key.pem
```

### Tags

A set of tags may be defined to associate additional key/value data with a host
when connecting to the broker. To do this, create the file
`/etc/yggdrasil/tags.toml` (assuming `SYSCONFDIR=/etc`, as the example above).
The contents of the file may be any number of TOML key/value pairs. However, a
limited number of TOML values are accepted as tag values (strings, integers,
booleans, floats, Local Date, Local Time, Offset Date-Time and Local Date-Time).
An example the tags.toml file can be found at at
/usr/share/doc/yggdrasil/tags.toml

## Running

yggdrasil uses D-Bus as an IPC framework to enable communication between workers
and the dispatcher, as well as exporting an API enabling other services to
interact with it. `yggd` follows the normal logic for determining which bus to
connect to. It will attempt to connect to the session bus if
`DBUS_SESSION_BUS_ADDRESS` is defined. Otherwise, it will attempt to connect to
the system bus. On the system bus, if the process is not running as root, the
installed D-Bus security policy will deny the process from claiming the
`com.redhat.Yggdrasil1` and `com.redhat.Yggdrasil1.Dispatcher1` names, and the
process will exit.

The systemd unit `yggrasil.service` starts `yggd`, using the logic described
above.

```
systemctl enable --now yggdrasil
```

Multiple instances of `yggd` can be run by using the templated systemd units.
Each instance requires  a private D-Bus session. The socket and broker are
started with `yggdrasil-bus@.socket` and `yggdrasil-bus@.service`, respectively.
A templated yggdrasil unit is available to automatically connect yggd to the
D-Bus broker started by `yggdrasil-bus@.service`.

```
systemctl enable --now yggdrasil-bus@bunnies.socket
systemctl enable --now yggdrasil-bus@bunnies.service
systemctl enable --now yggdrasil@bunnies.service
```

This will define and create an abstract UNIX domain socket named `yggd_bunnies`.
It will start a `dbus-broker` process running in the "user" scope, connecting to
the `yggd_bunnies` socket defined previously. Finally, `yggd` is launched with a
specific configuration file as the value of the `--config` argument:
`/etc/yggdrasil/yggdrasil-bunnies.toml`.

## Workers

A functional worker program must connect to the message bus as determined by the
`DBUS_SESSION_BUS_ADDRESS` environment variable, connecting to a session bus if
the value is defined, otherwise connecting to the system bus. Once connected to
the bus:

* The program must export an object on the bus that implements the
  `com.redhat.Yggdrasil1.Worker1` interface.
* The object must be exported at a path under  `/com/redhat/Yggdrasil1/Worker1`
  that includes the directive name (i.e. `/com/redhat/Yggdrasil1/Worker1/echo`).
* The worker must claim a well-known name that begins with
  `com.redhat.Yggdrasil1.Worker1` and includes its directive as the final segment
  in reverse-domain-name notation (i.e. `com.redhat.Yggdrasil1.Worker1.echo`).

A worker can transmit data back to a destination by calling the
`com.redhat.Yggdrasil1.Dispatcher1.Transmit` method.

Package `worker` implements the above requirements implicitly, enabling workers
to be written without needing to worry about much of the D-Bus requirements
outlined above.

See `worker/echo` for a reference implementation of a worker program.

## Integration tests

The Python integration tests under `integration-tests/` are configured through
[pytest-client-tools](https://github.com/RedHatInsights/pytest-client-tools)
(dynaconf). Non-secret configuration is downloaded at test time from the HTTP
server selected by the `SETTINGS_URL` variable. **Cutover status (dated): the
served `settings.toml` *is* the sanitized sentinel file.** The owner removed
the live unsanitized `settings.toml` from the settings server and published
the sanitized sentinel file at the canonical `SETTINGS_URL` on **2026-11-16**,
as part of the CCT-3155 migration. The published file carries 29 `@vault:`
sentinel values and zero plaintext secrets (md5
`47f79e04d195a3f6b6f999fcb26517bb`); the staged `settings.vault.toml` name on
the same server serves byte-identical content. These structure facts were
verified 2026-11-19; re-verify if the served file rotates. Secret values are stored
neither in the repository nor on the settings server: the in-repo master of
the sanitized file is `src/resources/config/settings.toml` in the rhsm-jobs
repository, landing with rhsm-jobs !557 (the repository is the master, the
settings server only its deployment target).

### How CI obtains credentials

In CI, secret values in the served `settings.toml` are marked as `@vault:`
sentinels. pytest-client-tools resolves each sentinel at config-load time by
authenticating to Vault with the AppRole delivered to the test guest as
encrypted Testing Farm secrets — `VAULT_ADDR`, `VAULT_ROLE_ID` and
`VAULT_SECRET_ID`, whose ciphertexts live in `.testing-farm.yaml`. Ciphertext
*is* stored in the repository; the plaintext values are not. A ciphertext only
decrypts for a request submitted for this repository with the Testing Farm
token that encrypted it. For Packit-submitted requests this protection is
structural against forks: a fork PR is submitted with the fork's own
repository URL, so these blobs do not decrypt. For the remaining submitters —
the team's Jenkins/direct submissions (`cct-ci` token) and the OSCI gating
runs — the repository URL and ref they submit against are enforced outside
this repository: the team's Jenkins pipelines and the OSCI gating
configuration submit against this repository's URL with reviewed refs (see
the PR description's "Trust model for the delivered AppRole" for scope and
lifecycle).

One non-secret variable guards the Vault TLS handshake itself:
`REQUESTS_CA_BUNDLE=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem`.
pytest-client-tools reaches Vault through Python `requests`, which verifies
TLS against the certifi bundle shipped alongside the pip-installed package —
a bundle that lacks the Red Hat Internal Root CA — so without this variable
the AppRole login fails its TLS handshake. The value is the guest's OS trust
bundle, which does contain the Red Hat Internal Root CA; it is not a secret.
It is delivered by the Jenkins pipelines pipeline-side for Jenkins-submitted
runs, and by `.packit.yaml` via `tf_extra_params` for the `unit/rhel` job on
Packit-submitted internal-ranch runs. The guest-image property — that this
exact path exists on RHEL-10 guests and contains the internal root CA — is
verified rather than assumed, with the evidence scoped per repository: on
RedHatInsights/insights-client#744 the same-delivery guest log shows the
Vault AppRole login completing with this exact path on RHEL-10 guests (HTTP
200); on RedHatInsights/rhc#656 the same-delivery run proves `@vault:`
sentinel resolution under strict mode (tests collected and ran rather than
erroring at config load), but its published artifacts carry no Vault HTTP
status line.

Not every committed CI job receives these secrets. The public-ranch Packit
jobs (`unit/centos-stream` and `unit/fedora` in `.packit.yaml`) deliberately
get none of them: no ciphertext exists for the public ranch's Packit token,
so the public ranch cannot decrypt anything. Those jobs run the same plan
with `SETTINGS_URL` unset and the credentialed external-Candlepin tests skip
by design; the non-credentialed tests still run (5 passed / 1 skipped on
this head's public-ranch runs). Only the internal-ranch job (`unit/rhel`,
`use_internal_tf: true`) receives the secrets and exercises the credentialed
path.

`systemtest/tests/integration/test.sh` exports
`PYTEST_CLIENT_TOOLS_VAULT_REQUIRED=1` ("strict mode"): an `@vault:` sentinel
that cannot be resolved — Vault unreachable, credentials not delivered, path
missing — fails the run at config load instead of silently degrading to a
missing value. Since the 2026-11-16 publication the served file carries the
`@vault:` sentinels, so strict mode is **not** inert on credentialed runs:
a run that receives the settings file but whose Vault credentials are
missing, wrong, or unusable fails at config load instead of skipping. Only
a run with no settings file at all (`SETTINGS_URL` unset) has no sentinel to
resolve — that case is caught separately by the `REQUIRE_SETTINGS` gate
below. Without strict mode, an unresolved sentinel leaves the value absent
(with a warning) and the tests that need it skip rather than fail. The Vault
namespace, address and secret minting are internal; see the CCT QE wiki
how-to referenced in `.testing-farm.yaml`.

Strict mode cannot fire when `SETTINGS_URL` is entirely unset — no settings
file means no sentinel to resolve — so a run whose secret delivery never
arrived would degrade to `1 skipped`, exit 0. The `REQUIRE_SETTINGS` gate in
`test.sh` covers that case: with `REQUIRE_SETTINGS=1`, an unset
`SETTINGS_URL` is a hard error naming the missing Testing Farm secret
delivery. Whether a run is gated depends on the submitter:

- **Packit internal** (`unit/rhel`, `use_internal_tf: true`): **gated**.
  `.packit.yaml` delivers `REQUIRE_SETTINGS=1` to the job's Testing Farm
  environment via `tf_extra_params` (`environments[].variables`), so a
  failed secret delivery fails the run.
- **Packit public** (`unit/centos-stream`, `unit/fedora`): **by-design
  skip**. These jobs receive no Testing Farm secrets (no ciphertext exists
  for the public ranch's Packit token), so `SETTINGS_URL` is always unset;
  the credentialed external-Candlepin tests skip by design, while the
  non-credentialed tests still run (5 passed / 1 skipped on this head's
  public-ranch runs).
- **Jenkins/direct (`cct-ci` token) and OSCI**: **currently fail-open**.
  These submitters have ciphertexts in `.testing-farm.yaml` and receive the
  secrets, but nothing pipeline-side delivers `REQUIRE_SETTINGS` to them,
  so a failed secret delivery still degrades to the clean skip rather than
  failing the run. Delivering `REQUIRE_SETTINGS` pipeline-side for these
  submitters is scheduled with the CCT-3165 cutover knob-setting (the same
  window that sets the strict-mode knobs).

Local and manual runs set no `REQUIRE_SETTINGS` and keep the documented
clean skip.

### Local and manual runs: environment variables

Outside CI — local or manual container runs — credentials can be supplied as
`PYTEST_CLIENT_TOOLS_*` environment variables, which take precedence over
`settings.toml`:

| Environment variable | Purpose |
|----------------------|---------|
| `PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME` | Candlepin (subscription-manager) username |
| `PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD` | Candlepin (subscription-manager) password (secret) |
| `PYTEST_CLIENT_TOOLS_CANDLEPIN__ACTIVATION_KEYS` | Candlepin activation keys (secret) |

The mapping rule is `PYTEST_CLIENT_TOOLS_<SECTION>__<KEY>` for nested keys and
`PYTEST_CLIENT_TOOLS_<KEY>` for top-level keys, with each `.` replaced by `__`
and the whole name uppercased. Values are parsed as TOML, and environment
variables take precedence over `settings.toml`. The `ENV_FOR_DYNACONF`
variable selects the environment (`default`, `stage`, `prod`, …) matching the
environments in `settings.toml`.

When `SETTINGS_URL` is set, `systemtest/tests/integration/test.sh` downloads
`settings.toml` from it before running the tests — a set but unreachable URL,
an HTTP error page, a redirect body, or any body that does not parse as TOML
fails the run; any previously downloaded file is kept as
`settings.toml.bak`. For local runs without `SETTINGS_URL`, a `settings.toml`
(and/or `.secrets.toml` for local secret overrides) can be placed in the
repository root; these files are git-ignored and must never be committed.
When `ENV_FOR_DYNACONF=testing` (manual runs against the containerized
Candlepin; no committed CI job sets this variable), `test.sh` falls back to
the container's well-known credentials if no explicit ones are set.

For example, to run the tests locally:

```bash
# 1. Provide settings.toml: either place your own (git-ignored) file in
#    the repository root, or download the sanitized one if you have the
#    URL (it is a Testing Farm secret; the CCT QE team can provide it):
curl -fsSL "$SETTINGS_URL" -o settings.toml
# 2. Select the environment and supply the secret values:
export ENV_FOR_DYNACONF=stage
read -s PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD
export PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD
# 3. Run the tests:
pytest -v integration-tests
```
