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

$ft --fail --mount :/proc:type=proc -- sh -c 'echo $$' &>/dev/null
test "$?" == 125 || {
  fail "faketree allowed to mount /proc despite --fail and no --proc option"
}

shpid=$($ft --fail -- sh -c 'echo $$')
test "$?" == 0 || {
  fail "faketree failed to run a simple 'echo $$'"
}
test "$shpid" != 1 || {
  fail "shell had pid of init in a dedicated pid namespace"
}

t1=$(mktemp $tmpdir/canary.XXXXXX)
t2=$(mktemp $tmpdir/canary.XXXXXX)
$ft --fail -- sh -c "(sleep 1; echo ready > $t1) & (sleep 2; echo ready > $t2) & exit 17"
test "$?" == 17 || {
  fail "faketree did not propagate error status correctly"
}
grep ready "$t1" &>/dev/null || {
  fail "faketree completed before the first canary file was created? $t1 does not exist"
}
grep ready "$t2" &>/dev/null || {
  fail "faketree completed before the second canary file was created? $t2 does not exist"
}

# Check that we get the correct exit status even when inner processes
# are killed with signals. A bit of a hack to find it (hint: grep on lockfile),
# wait for it (content of lockfile), and make sure the entire process group
# dies (the short sleep inside, sleep is a subcommand).
lock=$(mktemp $tmpdir/lock.XXXXXX)
$ft --fail -- bash -c "echo 'ready' > $lock; while :; do sleep 0.5; done;" &
while grep -L 'ready' $lock &>/dev/null; do sleep 0.2; done;
kill -SEGV $(pgrep -nf $lock)
wait %1
status="$?"
test "$status" == "139" || {
  fail "faketree did not propagate error status correctly - got $status"
}

# Blast faketree with SIGTERM, command ignoring it should still complete, faketree
# should keep waiting as if nothing happened.
lock=$(mktemp $tmpdir/lock.XXXXXX)
$ft --fail -- bash -c "trap '' TERM; echo 'started' > $lock; sleep 2; echo 'ready' > $lock" &
# Until bash gets to the "trap ''..." it is vulnerable to signals, it will die.
# Wait until it is safe to do so before blasting it with SIGTERM.
while grep -L 'started' $lock &>/dev/null; do sleep 0.2; done;
for r in {1..100}; do
  kill -TERM %1
done
wait %1
status="$?"
test "$status" == "0" || {
  fail "faketree was killed before completion? or failed? status $status"
}
grep ready "$lock" &>/dev/null || {
  fail "faketree lock $lock file was not written correctly into."
}

# Check that timeout is working as expected.
# Note the inner shell () being backgrounded: the main shell returns immediately,
# but the inner one remains running, for up to an hour.
#
# If timeout works, this command will complete in ~one second with status 12.
$ft --fail --wait-timeout=1s -- bash -c "(sleep 3600) &>/dev/null & exit 12"
test "$?" == "12" || {
  fail "faketree did not return the status of the main command"
}
