#!/bin/bash

set -e

LINUXIMG=$1
ROOTFS=$2
HOSTFS=$(dirname $(realpath $3))
KUNIT_TAP_PARSER=$4

export TMPDIR=$(mktemp -d)
OUTFILE="${TMPDIR}/output"

"$LINUXIMG" ubd0="$ROOTFS" hostfs="$HOSTFS" | tee "$OUTFILE"
"$KUNIT_TAP_PARSER" parse < "$OUTFILE"
