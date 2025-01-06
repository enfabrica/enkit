#!/usr/bin/env bash

set -o pipefail -o errexit -o nounset

readonly DIVE_BIN="{{dive_bin}}"
readonly TARBALL="{{tarball}}"

"$DIVE_BIN" "docker-archive://$TARBALL"
