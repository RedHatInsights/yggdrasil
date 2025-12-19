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
    "subscription-manager"
    "rhc"
  )
fi

# For CentOS Stream, add rhc and subscription-manager
# These are needed for integration tests that use pytest-client-tools
if [ "$ID" == "centos" ]; then
  packages+=(
    "rhc"
    "subscription-manager"
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
  # Print yggd version
  echo "yggd version"
  yggd --version

  # Print information about installed yggdrasil RPM package
  echo "yggdrasil RPM installed:"
  rpm -qi yggdrasil

  # Configure yggdrasil service to use local mosquitto MQTT broker
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

get_image_name() {
  if command -v jq > /dev/null; then
    IMAGE=$(bootc status --format=json | jq -r '.status.booted.image.image.image')
  else
    IMAGE=$(bootc status --format=humanreadable | grep 'Booted image' | cut -d' ' -f 4)
  fi
  echo "$IMAGE"
}

is_bootc() {
  command -v bootc > /dev/null &&
    ! bootc status --format=humanreadable | grep -q 'System is not deployed via bootc'
}

if is_bootc; then
  echo "info: running in bootc/image-mode, preparing new image"
  # TODO: fix for non testing-farm image mode environments
  IMAGE=$(get_image_name)
  echo "info: current image is $IMAGE"

  (podman pull $IMAGE || podman pull containers-storage:$IMAGE) || bootc image copy-to-storage --target $IMAGE
  podman build --build-arg IMAGE=$IMAGE -t localhost/yggdrasil-test:latest -f Containerfile systemtest/

  echo "info: switching to new bootc image and rebooting"
  bootc switch --transport containers-storage localhost/yggdrasil-test:latest
else
  echo "info: installing dependencies"
  install_epel
  setup_yggdrasil
  echo "info: dependencies installed successfully"
fi
