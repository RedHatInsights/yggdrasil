#!/bin/bash
set -x

is_bootc() {
  command -v bootc > /dev/null &&
    ! bootc status --format=humanreadable | grep -q 'System is not deployed via bootc'
}

if is_bootc; then
  if [ ! -f /test-deps-installed ]; then
    echo "info: marking test dependencies as installed to prevent reboot loop"
    tmt-reboot
  else
    echo "info: test dependencies already marked as installed"
  fi
else
  echo "info: not a bootc system, skipping post-reboot setup"
fi
