#!/bin/bash

set -e

LINUXIMG=$1
ROOTFS=$2
HOSTFS=$(dirname $(realpath $3))

export TMPDIR=$(mktemp -d)
OUTFILE="${TMPDIR}/output"
KUNIT_TAP_PARSER=/path/to/kunit_tap_parser

"$LINUXIMG" ubd0="$ROOTFS" hostfs="$HOSTFS" | tee "$OUTFILE"
"$KUNIT_TAP_PARSER" parse < "$OUTFILE"
