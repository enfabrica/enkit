#!/bin/bash

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
GEE="${SCRIPT_DIR}/gee"
TIMESTAMP="$(date +%s)"

# Clean test environment.
cd
rm -rf ~/testgee
export TESTMODE=1
declare ERRORS=0
declare CHECKS=0

function _cmd() {
  printf 2>&1 " %q" "$@"
  "$@"
}

function _gee() {
  _cmd "${GEE}" "$@"
}

function _expect_test {
  CHECKS=$((CHECKS+1))
  printf >&2 "EXPECT:";
  printf >&2 " %q" "$@"
  printf >&2 " ... "
  if [[ "$@" ]]; then
    printf >&2 "ok\n"
  else
    printf >&2 "ERROR\n"
    ERROR=$(( ERROR + 1 ))
  fi
}

function _expect_cmd {
  CHECKS=$((CHECKS+1))
  printf >&2 "EXPECT:";
  printf >&2 " %q" "$@"
  printf >&2 " ... "
  if "$@"; then
    printf >&2 "ok\n"
  else
    printf >&2 "ERROR\n"
    ERROR=$(( ERROR + 1 ))
  fi
}

function _expect_gee() {
  CHECKS=$((CHECKS+1))
  local RC
  _gee "$@"; RC="$?"
  if (( "${RC}" != 0 )); then
    printf >&2 "ERROR: RC=${RC}\n"
    ERROR=$((ERROR + 1))
  fi
}


_expect_gee init
_expect_test -d ~/testgee
_expect_test -d ~/testgee/.gee
_expect_test -d ~/testgee/github-playground/test-branch

cd ~/testgee/github-playground/test-branch
_expect_gee mkbr b
_expect_test -d ~/testgee/github-playground/b
cd ~/testgee/github-playground/b

mkdir geetest
mkdir "geetest/${TIMESTAMP}"
echo "a b c d e f g" > "geetest/${TIMESTAMP}/file1.txt"
echo "h i j k l m n" >> "geetest/${TIMESTAMP}/file1.txt"
_expect_gee commit -a -m "Adding file1.txt"

_expect_gee mkbr c
cd ../c
_expect_test -f "geetest/${TIMESTAMP}/file1.txt"

# cleanup
"
