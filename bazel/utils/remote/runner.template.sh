#!/bin/bash

RSYNC_OPTS={rsync_opts}
SSH_OPTS={ssh_opts}
TARGET_OPTS={target_opts}
MACHINES={machines}
ONLY_COPY={only_copy}
SSH_BIN={ssh_bin}
RSYNC_BIN={rsync_bin}

help() {
  echo "HELP MESSAGE - TODO"
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
MACHINES+=("$@")

[ "${#MACHINES[@]}" -ge 1 ] || {
  help "You must specify one or more machines to execute on"
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
  printrun "$RSYNC_BIN" --files-from="$include" "${RSYNC_OPTS[@]}" .. "$machine:$destpath/"
done

command="cd $destpath/$workspace; ./$executable ${TARGET_OPTS[*]}"
[ "$ONLY_COPY" != "true" ] || {
  echo "Copy only mode was requesting - not running any command"
  echo "Would have run:"
  echo "    $command" 
  exit 0
}

echo "Running '$command' on ${MACHINES[0]}..."
printrun "$SSH_BIN" "${SSH_OPTS[@]}" "${MACHINES[0]}" -- "$command"
