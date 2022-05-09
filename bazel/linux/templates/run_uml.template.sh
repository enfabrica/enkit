#!/bin/bash

set -e

# REL* are relative bazel paths.
# RUNTIME points to the root directory.
# INIT is relative to the root directory, points to the init script.
RELINIT="{init}"
RELRUNTIME="{runtime}"
ROOTFS="{rootfs}"
KERNEL="{kernel}"

# A script in charge of verifying the output of the run.
CHECKER="{checker}"

# Variables provided by bazel will point to a directory that only contains the
# deps for this targets as symlinks. But symlinks don't work if we only mount
# a subdirectory. This finds the original/underlying location.
INIT="$(realpath "$RELRUNTIME/$RELINIT")"
RUNTIME="${INIT%%$RELINIT}"

# Make sure the log file is saved by BES protocol, store it in the
# UNDECLARED_OUTPUTS_DIR.
TMPOUTPUT="$TEST_UNDECLARED_OUTPUTS_DIR/console.log"

# The Bazel TEST_TMPDIR path tends to get long depending on the project
# structure and the current working directory at the time of test invocation.
# This overflows UNIX_PATH_MAX that UML sees when firing up. To prevent this,
# create a temporary directory and link it under the Bazel tree so it gets
# cleaned up properly after the test.
UML_DIR=$(mktemp -u)
ln -sf "${TEST_TMPDIR}" "${UML_DIR}"

OPTIONS=()
if [ -n "$ROOTFS" ]; then
  OPTIONS+=("ubd0=$ROOTFS" "hostfs=$RUNTIME")
  RUNINIT="something in /etc/init.d or systemd running $RELINIT after mount"
else
  OPTIONS+=("rootfstype=hostfs" "init=$INIT")
  RUNINIT="running $RELRUNTIME/$RELINIT"
fi

echo 1>&2 "Kernel: $KERNEL"
echo 1>&2 "Rootfs: $ROOTFS"
echo 1>&2 "Runtime: $RUNTIME"
echo 1>&2 "Init: $INIT"
echo 1>&2 "======================================"

# If bazel is invoked as "bazel run" instead of "bazel test", throw the
# user in a shell rather than run the test.
if [ -n "$BUILD_WORKSPACE_DIRECTORY" ]; then
  "$KERNEL" con=pty con0=fd:0,fd:1 uml_dir="${UML_DIR}" "${OPTIONS[@]}" init=/bin/sh "$@" </dev/tty >/dev/tty || true
  exit
fi

"$KERNEL" con0=null,fd:1 con1=null,fd:1 uml_dir="${UML_DIR}" "${OPTIONS[@]}" "$@" | tee "$TMPOUTPUT"

test -z "$CHECKER" || {
  "$CHECKER" "$TEST_UNDECLARED_OUTPUTS_DIR" || {
    status="$?"
    echo 1>&2 "You can also use:"
    echo 1>&2 "   bazel run ${TEST_TARGET}"
    echo 1>&2
    echo 1>&2 "... to be dropped in a shell. Test is started by $RUNINIT".
    echo 1>&2 "Additional arguments after 'bazel run ... -- ' are passed unchanged"
    echo 1>&2 "to uml - you can use flags to control debugging, for example."
    exit "$status"
  }
}
