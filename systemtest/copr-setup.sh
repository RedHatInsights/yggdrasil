#!/usr/bin/bash -eux

dnf install -y dnf-plugins-core

# Determine the repo needed from copr
# Example of available repositories: 'rhel-8-x86_64', 'centos-stream-9-x86_64'
# 'centos-stream-10-x86_64', 'rhel-9-x86_64', 'fedora-40-x86_64',
# 'fedora-41-x86_64', 'fedora-rawhide-x86_64'
source /etc/os-release

VERSION_MAJOR=$(echo "${VERSION_ID}" | cut -d '.' -f 1)

if [[ "$ID" == "centos" ]] || { [[ "$ID" == "rhel" ]] && [[ "$VERSION_MAJOR" == "10" ]]; }; then
  ID='centos-stream'
fi

COPR_REPO="${ID}-${VERSION_MAJOR}-$(uname -m)"

dnf --setopt install_weak_deps=False install -y \
  podman git-core python3-pip python3-pytest logrotate mosquitto

# Start the mosquitto service
systemctl start mosquitto.service

# These PR packit builds have an older version number for some reason than the released...
dnf remove -y --noautoremove yggdrasil
dnf copr -y enable packit/RedHatInsights-yggdrasil-"${ghprbPullId}" "${COPR_REPO}"
dnf install -y yggdrasil --disablerepo=* --enablerepo=*yggdrasil*

# Configure yggdrasil service to use local mosquitto MQTT broker
cat <<'EOF' > /etc/yggdrasil/config.toml
# yggdrasil global configuration settings
protocol = "mqtt"
server = ["tcp://localhost:1883"]
log-level = "debug"
EOF
