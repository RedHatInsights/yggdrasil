#!/bin/bash

# Try to go to MESON_DIST_ROOT if this env. variable is defined
# and the directory exists
if [[ -n "${MESON_DIST_ROOT}" && -d "${MESON_DIST_ROOT}" ]]; then
pushd "${MESON_DIST_ROOT}" || exit 1
fi

go mod vendor

if [[ -n "${MESON_DIST_ROOT}" && -d "${MESON_DIST_ROOT}" ]]; then
popd || exit 1
fi
