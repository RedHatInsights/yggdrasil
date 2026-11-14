#!/bin/bash
set -ux

# get to project root
cd ../../../

# Start the mosquitto service
systemctl start mosquitto.service

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate
pip install --upgrade pip
pip install -r integration-tests/requirements.txt

# Non-secret test configuration is downloaded from the HTTP server
# selected by SETTINGS_URL (the served file is sanitized). Secret values
# in the served file are @vault: sentinels that pytest-client-tools
# resolves at config load, using the Vault AppRole delivered to the guest
# as Testing Farm secrets (VAULT_ADDR, VAULT_ROLE_ID, VAULT_SECRET_ID).
# For local runs, the PYTEST_CLIENT_TOOLS_* environment variables
# documented in README.md ("Integration tests") can be used instead.
# Strict mode: an @vault: sentinel that cannot be resolved fails the run
# at config load instead of degrading to a missing value. Inert while
# the served file carries no sentinels.
export PYTEST_CLIENT_TOOLS_VAULT_REQUIRED=1
# REQUIRE_SETTINGS gate: strict mode cannot fire when SETTINGS_URL is
# entirely unset (no settings file means no sentinel to resolve), so a
# secret-delivery failure would degrade to "1 skipped", exit 0. The
# internal-ranch Packit job (unit/rhel) sets REQUIRE_SETTINGS=1 in the
# Testing Farm environment via tf_extra_params in .packit.yaml; when it
# is set, an unset SETTINGS_URL is a hard error. Runs without the gate
# (local/manual runs; the public-ranch jobs, which receive no Testing
# Farm secrets by design) keep the documented clean skip.
if [ "${REQUIRE_SETTINGS:-}" = "1" ] && [ -z "${SETTINGS_URL:-}" ]; then
  echo "ERROR: REQUIRE_SETTINGS=1 but SETTINGS_URL is unset: the Testing Farm secret delivery (SETTINGS_URL, VAULT_ADDR, VAULT_ROLE_ID, VAULT_SECRET_ID) did not reach this guest; failing instead of skipping the credentialed tests" >&2
  exit 1
fi
# curl -fsSL rejects HTTP error pages and follows redirects instead of
# saving a redirect or error body as settings.toml, the downloaded file
# must parse as TOML, and a set-but-broken SETTINGS_URL fails the run:
# a configured URL must never silently degrade to running without the
# settings file.
if [ -n "${SETTINGS_URL+x}" ]; then
  [ -f ./settings.toml ] && mv ./settings.toml ./settings.toml.bak
  if ! curl -fsSL "$SETTINGS_URL" -o ./settings.toml; then
    echo "ERROR: Failed to download settings from: $SETTINGS_URL" >&2
    exit 1
  fi
  if ! python3 -c 'import sys, tomllib; tomllib.load(open(sys.argv[1], "rb"))' ./settings.toml; then
    echo "ERROR: $SETTINGS_URL did not serve a valid TOML settings file" >&2
    exit 1
  fi
fi

# The sanitized settings file provides candlepin.username without the
# password, and pytest-client-tools requires a password (or activation keys)
# whenever a username is set. For manual runs against the containerized
# candlepin (ENV_FOR_DYNACONF=testing; no committed CI job sets it) fall
# back to the container's well-known credentials; credentials for real
# environments must be supplied through PYTEST_CLIENT_TOOLS_* environment
# variables.
if [ "${ENV_FOR_DYNACONF:-}" = "testing" ]; then
  # Suppress tracing: the log pipeline masks only the values it injected
  # itself, so an operator-supplied real password must not reach the log.
  set +x
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME="${PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME:-admin}"
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD="${PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD:-admin}"
  set -x
fi

pytest --junit-xml=./junit.xml -v integration-tests
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  if [ -d ./artifacts ]; then
    cp -r ./artifacts "$TMT_PLAN_DATA/"
  else
    echo "no ./artifacts directory exist"
  fi
fi

exit $retval
