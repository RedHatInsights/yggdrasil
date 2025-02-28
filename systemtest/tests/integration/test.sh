#!/bin/bash
set -ux

# get to project root
cd ../../../

if [[ "$ID" == "centos" ]] || { [[ "$ID" == "rhel" ]] && [[ "$VERSION_MAJOR" == "10" ]]; }; then
  dnf config-manager --set-enabled crb || true
  dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-10.noarch.rpm
fi

dnf --setopt install_weak_deps=False install -y \
  yggdrasil podman git-core python3-pip python3-pytest logrotate mosquitto

# Start the mosquitto service
systemctl start mosquitto.service

# Print yggd version
echo "yggd version"
yggd --version

# Print information about installed yggdrasil RPM package
echo "yggdrasil RPM installed:"
rpm -qi yggdrasil

# Configure yggdrasil service to use local mosquitto MQTT broker
cat <<'EOF' > /etc/yggdrasil/config.toml
# yggdrasil global configuration settings
protocol = "mqtt"
server = ["tcp://localhost:1883"]
log-level = "debug"
EOF

# Check for bootc/image-mode deployments which should not run dnf
if ! command -v bootc >/dev/null || bootc status | grep -q 'type: null'; then
  echo "warning: running in bootc/image-mode"
  if [ -z "${ghprbPullId+x}" ] ; then
    ./systemtest/copr-setup.sh
  else
    echo "The ./systemtest/copr-setup.sh is not used, because env. var. 'ghprbPullId' is not set"
fi

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate

pip install -r integration-tests/requirements.txt

pytest --junit-xml=./junit.xml -v integration-tests
retval=$?

if [ -d "$TMT_PLAN_DATA" ]; then
  cp ./junit.xml "$TMT_PLAN_DATA/junit.xml"
  cp -r ./artifacts "$TMT_PLAN_DATA/"
fi

exit $retval
