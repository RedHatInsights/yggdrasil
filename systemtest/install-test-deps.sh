#!/bin/bash
set -x

packages=(
  "git-core"
  "golang"
  "logrotate"
  "meson"
  "mosquitto"
  "podman"
  "python3-pip"
  "python3-pytest"
  "yggdrasil"
)

source /etc/os-release

VERSION_MAJOR=$(echo "${VERSION_ID}" | cut -d '.' -f 1)

if [ "$ID" == "rhel" ]; then
  packages+=(
    "insights-client"
  )
fi

install_epel() {
  if [[ "$ID" == "centos" ]] || [[ "$ID" == "rhel" ]]; then
    if [[ "$VERSION_MAJOR" == "10" ]]; then
      dnf config-manager --set-enabled crb || true
      dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-10.noarch.rpm
      dnf config-manager --set-enabled epel || true
    fi
    if [[ "$VERSION_MAJOR" == "9" ]]; then
      if ! rpm -qa | grep epel-release; then
        echo "The epel-release not installed"
        if [[ "${ID}" == "centos" ]]; then
          echo "Enabled CRB"
          dnf config-manager --set-enabled crb || true
        fi
        if [[ "${ID}" == "rhel" ]]; then
          echo "Enabled CodeReady repository"
          subscription-manager repos --enable "codeready-builder-for-rhel-9-$(arch)-rpms" || true
        fi
        dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
        dnf config-manager --set-enabled epel || true
      else
        echo "The epel-release already installed"
      fi
    fi
  fi
  dnf --setopt install_weak_deps=False install -y "${packages[@]}"
}

setup_yggdrasil() {
  echo "yggd version"
  yggd --version

  echo "yggdrasil RPM installed:"
  rpm -qi yggdrasil

  cat << 'EOF' > /etc/yggdrasil/config.toml
# yggdrasil global configuration settings
protocol = "mqtt"
server = ["tcp://localhost:1883"]
log-level = "debug"
path-prefix = "yggdrasil"
EOF

  # Install the Echo worker for downstream tests.
  # Note: The Echo worker is not currently built  in yggdrasil,
  # but it is available in upstream COPR builds as part of the packaged distribution.
  if [ ! -x /usr/libexec/yggdrasil/echo ]; then
    mkdir -p /usr/libexec/yggdrasil
    TEMP_HOME=$(mktemp -d)
    HOME=$TEMP_HOME go install github.com/redhatinsights/yggdrasil/worker/echo@latest
    cp $TEMP_HOME/go/bin/echo /usr/libexec/yggdrasil/echo
    rm -rf $TEMP_HOME

    yggctl generate worker-data --name echo --program /usr/libexec/yggdrasil/echo --user yggdrasil --output dbusfile_worker
    cp dbusfile_worker/dbus-1/system.d/com.redhat.Yggdrasil1.Worker1.echo.conf /usr/share/dbus-1/system.d/
    cp dbusfile_worker/systemd/system/com.redhat.Yggdrasil1.Worker1.echo.service /usr/lib/systemd/system/com.redhat.Yggdrasil1.Worker1.echo.service
    cp dbusfile_worker/dbus-1/system-services/com.redhat.Yggdrasil1.Worker1.echo.service /usr/share/dbus-1/system-services/com.redhat.Yggdrasil1.Worker1.echo.service
  fi
}

install_epel
setup_yggdrasil
