#!/bin/bash

function cleanFiles() {
  declare -A MAP=()
  local L1 L2 L3
  while IFS="" read -r L1; do
    IFS="" read -r L2
    IFS="" read -r L3
    if ! [[ -v MAP[L1] ]]; then
      MAP["$L1"]=1
      printf "%s\n%s\n%s\n" "${L1}" "${L2}" "${L3}"
    fi
  done < <(cat "$@")
}

if [[ "$STRATEGY" == "ALL" ]]; then
  cleanFiles "$@" >> "$OUT"
  exit 0;
fi
if [[ "$STRATEGY" == "git" ]]; then
  GIT_FILE_PATH="$(find "$PWD" -name "bin")"
#  combining them doesn't work, have 0 idea why
  REAL_GIT="$(find "$GIT_FILE_PATH" -name "$GIT_FILE")"
  if [ -z "$GIT_FILE_PATH" ]; then
      echo "No Git changes found"
      touch "$OUT"
      exit 0;
  fi
  x="$(cat "$REAL_GIT")"
  readarray -t arrs <<< "$x"
  noDupes=$(cleanFiles "$@")
  for i in ${arrs[*]}; do
    while IFS="" read -r L1; do
      IFS="" read -r L2
      IFS="" read -r L3
      if [[ "$L1" == *"$i"* ]]; then
        printf "%s\n%s\n%s\n" "${L1}" "${L2}" "${L3}" >> "$OUT"
      fi
    done < <(echo "$noDupes")
  done
fi
