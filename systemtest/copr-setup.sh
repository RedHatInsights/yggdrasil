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

# These PR packit builds have an older version number for some reason than the released...
dnf remove -y --noautoremove yggdrasil
dnf copr -y enable packit/RedHatInsights-yggdrasil-"${ghprbPullId}" "${COPR_REPO}"
dnf install -y yggdrasil --disablerepo=* --enablerepo=*yggdrasil*
