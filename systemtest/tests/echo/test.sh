#!/bin/sh

set -x

# Dump shell environment for context
env

# Reload DBus
busctl --system call org.freedesktop.DBus / org.freedesktop.DBus ReloadConfig

# Print relevant package info
rpm -qi yggdrasil

# Configure yggdrasil for local-dispatch only
cat << EOF > /etc/yggdrasil/config.toml
protocol = "none"
log-level = "debug"
message-journal = "/var/lib/yggdrasil/journal.sqlite3"
EOF

# Ensure yggdrasil is running
systemctl restart yggdrasil
systemctl status --full --no-pager yggdrasil
busctl --system status com.redhat.Yggdrasil1

# Ensure the echo worker is available
busctl --system status com.redhat.Yggdrasil1.Worker1.echo

# Locally dispatch a message to the echo worker
MESSAGE_UUID=$(echo '"hello"' | yggctl dispatch --worker echo - | cut -f3 -d " ")

# Sleep for a second to let the echo return
sleep 1

# Query the message-journal for MESSAGE_UUID
yggctl message-journal --message-id "${MESSAGE_UUID}" --format json
