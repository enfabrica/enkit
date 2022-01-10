#!/bin/bash

foundft="$(find . -name "faketree" -type f -executable)"
ft="./faketree/faketree_/faketree"
if [ ! -f $ft ]; then
  ft=$foundft
fi
function fail() {
  echo -- "$@" >&2
  exit 1
}

$ft --help &>/dev/null
test "$?" == 125 || {
  fail "--help should return error status 125 but it returned ${$?}"
}

uid=$($ft -- sh -c 'echo $(id -u)')
test "$?" == 0 || {
  fail "simple shell command failed"
}
test "$uid" == "$UID" || {
  fail "uid does not match expected uid - $uid"
}

uid=$($ft --root -- sh -c 'echo $(id -u)')
test "$?" == 0 || {
  fail "starting as root failed"
}
test "$uid" == "0" || {
  fail "running as root failed - $uid"
}

dir=$($ft --mount /etc:/tmp/root/etc --chdir /tmp/root -- sh -c pwd)
test "$?" == 0 || {
  fail "unexpected failure in simple mount"
}
test "$dir" == "/tmp/root" || {
  fail "chdir failed, poorly - $dir"
}

dir=$($ft --mount /etc:/tmp/root/etc --chdir /tmp/root/etc -- sh -c 'realpath `pwd`')
test "$?" == 0 || {
  fail "unexpected failure in simple mount"
}
test "$dir" == "/tmp/root/etc" || {
  fail "realpath pwd does not match - $dir"
}

$ft --mount /etc:/tmp/root/etc --chdir /tmp/root/etc -- sh -c 'exit 12' &>/dev/null
test "$?" == 12 || {
  fail "faketree was suppsoed to propagate error 12"
}

$ft --mount /etc:/tmp/root/etc --chdir /tmp/root/etc -- sh -c 'cat ./hosts' &>/dev/null
test "$?" == 0 || {
  fail "faketree could not mount /etc correctly - no hosts file??"
}

$ft --fail --mount /non-existing-path:/tmp/root/etc -- sh -c 'pwd' &>/dev/null
test "$?" == 125 || {
  fail "faketree did not fail with a non-existing directory!"
}

$ft -- sh -c 'pwd' &>/dev/null
test "$?" == 0 || {
  fail "faketree did not succeed even after the command was fixed"
}
