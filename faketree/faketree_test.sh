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
tmpdir=$(mktemp -d -t faketree-XXXXXXXXXX)
dir=$($ft --mount $tmpdir:/tmp/root/etc --chdir /tmp/root -- sh -c pwd)
test "$?" == 0 || {
  fail "unexpected failure in simple mount"
}
test "$dir" == "/tmp/root" || {
  fail "chdir failed, poorly - $dir"
}

dir=$($ft --mount $tmpdir:/tmp/root/etc --chdir /tmp/root/etc -- sh -c 'realpath `pwd`')
test "$?" == 0 || {
  fail "unexpected failure in simple mount"
}
test "$dir" == "/tmp/root/etc" || {
  fail "realpath pwd does not match - $dir"
}
$ft --mount $tmpdir:/tmp/root/etc --chdir /tmp/root/etc -- sh -c 'exit 12' &>/dev/null
test "$?" == 12 || {
  fail "faketree was suppsoed to propagate error 12"
}

tmpfile=$(mktemp $tmpdir/faketree.XXXXXX)
cmd="cat ./$tmpfile"
$ft --mount $tmpdir:/tmp/root/etc --chdir /tmp/root/etc -- sh -c $cmd &>/dev/null
test "$?" == 0 || {
  fail "faketree could not find $tmpfile correctly - no $tmpdir directory"
}

$ft --fail --mount /non-existing-path:/tmp/root/etc -- sh -c 'pwd' &>/dev/null
test "$?" == 125 || {
  fail "faketree did not fail with a non-existing directory!"
}

$ft -- sh -c 'pwd' &>/dev/null
test "$?" == 0 || {
  fail "faketree did not succeed even after the command was fixed"
}

$ft --fail --mount :/tmp/root/mytmp:ro,type=tmpfs -- sh -c "cat /proc/mounts" 2>/dev/null |egrep "tmpfs\s+/tmp/root/mytmp\s+tmpfs\s+ro"
test "$?" == 0 || {
  fail "faketree mounts do not show the just mounted tmpfs directory"
}
