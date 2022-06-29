#!/bin/bash

OTUDIR="${OUTDIR:-$(mktemp -d)}"
destfile="$OUTDIR/$1"

shift
echo "$@" >> "$destfile"
