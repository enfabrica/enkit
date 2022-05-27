#!/bin/bash

set -e

echo "===== Running target: {target}"
echo "===== Path: $(realpath $0)"

# REL* are relative bazel paths.
# RUNTIME points to the root directory.
# INIT is relative to the root directory, points to the init script.
RELINIT="{init}"
RELRUNTIME="{runtime}"
ROOTFS="{rootfs}"
KERNEL="{kernel}"
TARGET="{target}"

# A script in charge of verifying the output of the run.
CHECKER="{checker}"

function help {
  cat <<END
This script executes your target in an emulator.
Prefer to use as:

  bazel run $TARGET -- [-k kernel option]... [-e emu option]... [-r rootfs] [-s|-x|-h]

or:

  bazel build $TARGET
  bazel-bin/.../path/to/file [-k option]... [-e option]... [-s|-x|-h]

Accepted options:

  -s           Configures /bin/sh as init, drops you in a shell.
               Hint: you can then use the paths shown with -x to manually
               start the init script used.

  -k [value]   Adds one or more command line options to the kernel.

     For example: "-k ro -k root=/dev/sda -k console=ttyS0"
     will add "ro root=/dev/sda console=ttyS0" to the kernel command line.

  -e [value]   Adds one or more command line options to the emulator.

     For example: "-e'-f' -e/dev/sda -e'-mem' -e2048"
     will add "-f /dev/sda -mem 2048" to the emulator command line.

  -r [value]   Overrides the path to the rootfs.

     For example: "-r /tmp/myown.qcow" will ask the emulator to run
     the specified rootfs.

  -x           Prints info on the paths of the scripts used so you
               can use your immense wisdom to manually inspect and
               modify them.

  -h           Prints this astonishingly helpful message.

END
}

function onexit {
    status="$?"
    test "$status" != 0 || exit 0

    echo 1>&2 "====================================="
    echo 1>&2 "Use:"
    echo 1>&2 "   bazel run ${TARGET} -- -h"
    echo 1>&2
    echo 1>&2 "... to learn how to manually run this target for debugging, and"
    echo 1>&2 -e "override targets. Once in the VM, the test is started by $RUNINIT"
    echo 1>&2
    echo 1>&2 "use 'bazel run ${TARGET} -- -x' to see the full paths of scripts, so"
    echo 1>&2 "you can run them manually to debug."
    exit "$status"
}

function showstate {
    echo 1>&2 "CWD: $(realpath "$PWD")"
    echo 1>&2 "Script: $(realpath "$0")"
    echo 1>&2 "Kernel: $(realpath "$KERNEL")"
    echo 1>&2 "Rootfs: $ROOTFS"
    echo 1>&2 "Runtime: $RUNTIME"
    echo 1>&2 "Init: $INIT"
}

declare -a KERNEL_OPTS
declare -a EMULATOR_OPTS
INTERACTIVE=""
SINGLE=""

# Make sure the log file is saved by BES protocol, store it in the
# UNDECLARED_OUTPUTS_DIR.
TMPDIR="${TEST_TMPDIR:-$(mktemp -d)}"
OUTPUT_DIR=${TEST_UNDECLARED_OUTPUTS_DIR:-$(mktemp -d)}

# Variables provided by bazel will point to a directory that only contains the
# deps for this targets as symlinks. But symlinks don't work if we only mount
# a subdirectory. This finds the original/underlying location.
INIT="$(realpath "$RELRUNTIME/$RELINIT")"
RUNTIME="${INIT%%$RELINIT}"
OUTPUT_FILE="$OUTPUT_DIR/console.log"

while getopts "k:e:r:hsx" opt; do
  case "$opt" in
    h) help; exit 0;;
    k) KERNEL_OPTS+=("$OPTARG");;
    e) EMULATOR_OPTS+=("$OPTARG");;
    s) SINGLE=True;;
    r) ROOTFS=("$OPTARG");;
    x) showstate; exit 0;;
    ?|*) help 1>&2; exit 1;;
  esac
done
shift $((OPTIND - 1))

showstate

echo 1>&2 "======================================"
if [ -n "$ROOTFS" ]; then
  RUNINIT="something in init or systemd running\n\t    $RELINIT\nwherever that is mounted."
else
  RUNINIT="running\n\t    $RELRUNTIME/$RELINIT"
fi

trap onexit EXIT

# Contract with the included code:
# - It is run with 'set -e', any non-zero status causes exit with an error.
# - Code can only use the following variables:
#   - TARGET - name of the target running the rule.
#   - KERNEL - path to the kernel to run.
#   - INIT - path to the file to start as init/after init.
#   - ROOTFS - path to the root file system to use.
#   - RUNTIME - path to the top level directory that needs to be exposed.
#   - TMPDIR - path to a temporary directory.
#   - INTERACTIVE - if non-empty value, run the VM in interactive mode (shell).
#   - SINGLE - if non-empty value, run the VM in single-user mode (init=/bin/sh).
#   - OUTPUT_FILE - console output must be stored in this file
#                   (kernel boot log, and any shell output).
#   - OUTPUT_DIR - directory where to store any other output file.
#
# - Additionally, they should check for:
#   - KERNEL_OPTS - array, may have additional kernel arguments.
#   - EMULATOR_OPTS - array, may have additional arguments for the emulator.
for var in TARGET KERNEL INIT ROOTFS RUNTIME TMPDIR INTERACTIVE SINGLE OUTPUT_FILE OUTPUT_DIR KERNEL_OPTS EMULATOR_OPTS; do
    export "$var"
done

echo 1>&2 "======================================"
echo 1>&2 "Running preparations..."
{prepares}


echo 1>&2 "======================================"
echo 1>&2 "Running emulator..."
{code}

estatus="$?"
test "$estatus" == 0 || {
    echo 1>&2 "===== emulator exited with non zero status - check logs above ===="
    exit "$estatus"
}
test -f "$OUTPUT_DIR/exit_status_file" && estatus=$(< "$OUTPUT_DIR/exit_status_file")
test "$estatus" == 0 || {
    echo 1>&2 "===== a program in the emulator exited with non zero status - check logs above ===="
    exit "$estatus"
}
test -z "$INTERACTIVE" -a -z "$SINGLE" || exit "$estatus"

echo 1>&2 "===== emulator exited - now running checkers (if any) ===="
{checks}
