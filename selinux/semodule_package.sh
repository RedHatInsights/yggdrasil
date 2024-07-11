#!/bin/sh
# Copyright 2019 Red Hat Inc.
# Copyright 2022 Collabora Ltd.
# Copyright 2024 Red Hat Inc.
# SPDX-License-Identifier: LGPL-2.1-or-later

set -xeu

TMP=$(mktemp -d selinux-build-XXXXXX)
output="$1"
shift
cp -- "$@" "$TMP/"

make -C "$TMP" -f /usr/share/selinux/devel/Makefile "$(basename "$output")"
cp "$TMP/$(basename "$output")" "$output"
rm -fr "$TMP"
