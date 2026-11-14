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
# selected by SETTINGS_URL (the served file is sanitized). Secrets must
# be provided as PYTEST_CLIENT_TOOLS_* environment variables, which take
# precedence over settings.toml (see README.md, "Integration tests").
# curl -f rejects HTTP error pages instead of saving them as settings.toml,
# and a set-but-broken SETTINGS_URL fails the run: a configured URL must
# never silently degrade to running without the settings file.
if [ -n "${SETTINGS_URL+x}" ]; then
  [ -f ./settings.toml ] && mv ./settings.toml ./settings.toml.bak
  if ! curl -f "$SETTINGS_URL" -o ./settings.toml; then
    echo "ERROR: Failed to download settings from: $SETTINGS_URL" >&2
    exit 1
  fi
fi

# The sanitized settings file provides candlepin.username without the
# password, and pytest-client-tools requires a password (or activation keys)
# whenever a username is set. For the containerized candlepin
# (ENV_FOR_DYNACONF=testing) fall back to the container's well-known
# credentials; credentials for real environments must be supplied through
# PYTEST_CLIENT_TOOLS_* environment variables.
if [ "${ENV_FOR_DYNACONF:-}" = "testing" ]; then
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME="${PYTEST_CLIENT_TOOLS_CANDLEPIN__USERNAME:-admin}"
  export PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD="${PYTEST_CLIENT_TOOLS_CANDLEPIN__PASSWORD:-admin}"
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
