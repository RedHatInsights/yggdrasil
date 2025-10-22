#!/bin/bash
set -ux

# get to project root
cd ../../../

# Start the mosquitto service
systemctl start mosquitto.service

python3 -m venv venv
# shellcheck disable=SC1091
. venv/bin/activate

pip install -r integration-tests/requirements.txt

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
