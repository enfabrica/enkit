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
OUTPUTFILE="$TEST_UNDECLARED_OUTPUTS_DIR/console.log"

echo 1>&2 "Script: $(realpath $0)"
echo 1>&2 "Kernel: $KERNEL"
echo 1>&2 "Rootfs: $ROOTFS"
echo 1>&2 "Runtime: $RUNTIME"
echo 1>&2 "Init: $INIT"

if [ -n "$CHECKER" ]; then
    echo 1>&2 "Checker: $(realpath $CHECKER)"
else
    echo 1>&2 "Checker: <no checker configured>"
fi
echo 1>&2 "======================================"

if [ -n "$ROOTFS" ]; then
  RUNINIT="something in init or systemd running\n\t    $RELINIT\nwherever that is mounted."
else
  RUNINIT="running\n\t    $RELRUNTIME/$RELINIT"
fi

# Contract with the included code:
# - It is run with 'set -e', any non-zero status causes exit with an error.
# - Code can only use the following variables:
#   - KERNEL - path to the kernel to run.
#   - INIT - path to the file to start as init/after init.
#   - ROOTFS - path to the root file system to use.
#   - RUNTIME - path to the top level directory that needs to be exposed.
#   - TMPDIR - path to a temporary directory.
#   - INTERACTIVE - if non-empty value, run the VM in interactive mode (shell).
#   - OUTPUTFILE - console output must be stored in this file
#                  (kernel boot log, and any shell output).
INTERACTIVE="${INTERACTIVE:$BUILD_WORKSPACE_DIRECTORY}"
TMPDIR="${TEST_TMPDIR}"
{code}

test -z "$INTERACTIVE" || exit

test -z "$CHECKER" || {
  echo 1>&2 "===== emulator exited successfully - checking the results with $(realpath $CHECKER) ===="
  "$CHECKER" "$TEST_UNDECLARED_OUTPUTS_DIR" || {
    status="$?"
    echo 1>&2 "====================================="
    echo 1>&2 "You can also use:"
    echo 1>&2 "   bazel run ${TEST_TARGET}"
    echo 1>&2
    echo 1>&2 -e "... to be dropped in a shell. Test is started by $RUNINIT".
    echo 1>&2 "Additional arguments after 'bazel run ... -- ' are passed unchanged"
    echo 1>&2 "to the emulator"
    exit "$status"
  }
}
