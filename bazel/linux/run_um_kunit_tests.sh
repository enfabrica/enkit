#!/bin/bash

LINUXIMG=$1
ROOTFS=$2
HOSTFS=$(dirname $(realpath $3))

set -e

mkdir -p /tmp/uml
USER=$(whoami)
chown "${USER}.${USER}" /tmp/uml
chmod 777 /tmp/uml
export TMPDIR=/tmp/uml

OUTFILE="${TMPDIR}/output"
KUNIT_TAP_PARSER=/path/to/kunit_tap_parser

"$LINUXIMG" ubd0="$ROOTFS" hostfs="$HOSTFS" | tee $OUTFILE
KUNIT_TAP_PARSER parse < $OUTFILE
