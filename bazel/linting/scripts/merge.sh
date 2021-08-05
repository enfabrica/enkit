#!/bin/bash
if [[ "$STRATEGY" == "ALL" ]]; then
  echo "here"
  awk '!x[$0]++' "$@" >> "$OUT"
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
  echo "running ${arrs[*]}"
  HELLO="$(awk '!x[$0]++' "$@")"
  grep "${arrs[*]}" >> "$OUT"
  echo "hello" >> "$OUT"
fi
