#!/bin/bash

test -z "{debug}" || {
  echo "Running $0 in debug mode" 1>&2
  set -x
}

idir="$1"
odir="$2"

test "$#" -eq "2" || {
  echo "invalid command line: an input dir and an output dir must be provided" 1>&2
  exit 1
}

transform() {
  mkdir -p "$(dirname "$output")"
  {command}
}

include() {
  mkdir -p "$(dirname "$output")"
  cp -f "$input" "$output"
}

find "$idir" -not -type d | {
  while read line; do
    input="$line"
    output="${odir}${line#$idir/}"
    case "$line" in
    {patterns}
    esac
  done;
}
