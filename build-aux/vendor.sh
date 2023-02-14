#!/bin/bash

pushd "${MESON_DIST_ROOT}" || exit 1
go mod vendor
popd || exit 1
