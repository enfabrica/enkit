#!/bin/bash

RSYNC_OPTS={rsync_opts}
SSH_OPTS={ssh_opts}
TARGET_OPTS={target_opts}
MACHINES={machines}
ONLY_COPY={only_copy}
SSH_CMD={ssh_cmd}
RSYNC_CMD={rsync_cmd}

help() {
  test -z "$*" || {
    exec 1>&2
  } 

  cat <<END
This script runs a target on a remote machine, after copying all its dependencies.

Use as:

  bazel run $TARGET -- [-r rsync options]... [-s ssh options]... [-t target options]... [-o|-h] [MACHINE]...

Accepted options:

  MACHINE      List of machines to copy the target to. The target will then be
               run on the first machine specified, unless -o is used.

               The list of machines is MANDATORY, unless it is already specified
               in the rule in the BUILD.bazel file. In that case, if you pass any
               machine on the command line the list will REPLACE the one supplied
               in the BUILD.bazel file.

  -o           Don't run the binary, only copy all the files to the remote machines.

  -r [value]   Adds one or more rsync options.

     For example: "-r'-v' -r'--show-progress'" will append the "-v --show-progress"
     options to rsync.

  -t [value]   Adds one or more options to your target.

     For example: "-t'--baremetal'" will run the specified target with the "--baremetal"
     option. Those options are highly dependant on the target.

  -h           Prints this astonishingly helpful message.
END

  test -z "$*" || {
      echo
      echo "ERROR:" "$@"
      exit 10
  } 
  exit 0
}

while getopts "r:s:t:ho" opt; do
  case "$opt" in
    h) help;;
    r) RSYNC_OPTS+=("$OPTARG");;
    s) SSH_OPTS+=("$OPTARG");;
    t) TARGET_OPTS+=("$OPTARG");;
    o) ONLY_COPY=true;;
    *) break;;
  esac
done
shift $((OPTIND - 1))

[ "$#" -le 0 ] || {
  MACHINES=("$@")
}

[ "${#MACHINES[@]}" -ge 1 ] || {
  help "You must specify one or more MACHINE to copy the output to"
}

target={target}
include={include}
destdir={destdir}
destpath="$destdir/${target//[^a-zA-Z0-9_-]/_}"
executable={executable}
workspace={workspace}

printrun() { echo "+ $*"; "$@"; }

echo "Copying files..."
set -e
for machine in "${MACHINES[@]}"; do
  printrun $RSYNC_CMD --files-from="$include" "${RSYNC_OPTS[@]}" .. "$machine:$destpath/"
done

# TODO(cccontavalli): better escaping, will fix it once we have more tests.
command="cd $destpath/$workspace; MACHINES='${MACHINES[*]}' ./$executable ${TARGET_OPTS[*]}"
[ "$ONLY_COPY" != "true" ] || {
  echo "Copy only mode was requesting - not running any command"
  echo "Would have run:"
  echo "    $command" 
  exit 0
}

echo "Running '$command' on ${MACHINES[0]}..."
printrun $SSH_CMD "${SSH_OPTS[@]}" "${MACHINES[0]}" -- "$command"
