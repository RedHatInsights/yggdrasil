#!/bin/bash

USER_NAME="yggd"
SYS_USER_DIR=$(pkg-config systemd --variable=sysusersdir)
SYS_USER_CONF_FILE="${SYS_USER_DIR}"/"${USER_NAME}".conf

# Create system user for running yggd
if [[ -f "${DESTDIR}"/"${SYS_USER_CONF_FILE}" ]]; then
  systemd-sysusers --root "${DESTDIR}" "${DESTDIR}"/"${SYS_USER_CONF_FILE}"
else
  echo "Error: ${SYS_USER_CONF_FILE} does not exist. Cannot create sys user: ${USER_NAME}"
fi
