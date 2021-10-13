#!/bin/bash

function sum() {
  TOTAL=0
  for t in "$@"; do
    TOTAL=$(( TOTAL + t ))
  done
  echo "${TOTAL}"
}

function main() {
  echo "MAIN shouldn't be run by bats."
  exit 1
}

# only run main 
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi
