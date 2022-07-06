#!/bin/bash

RSYNC_OPTS={rsync_opts}
SSH_OPTS={ssh_opts}
TARGET_OPTS={target_opts}
DESTS={dests}
ONLY_COPY={only_copy}
NO_EXECUTE={no_execute}
SSH_CMD={ssh_cmd}
RSYNC_CMD={rsync_cmd}

help() {
  test -z "$*" || {
    exec 1>&2
  } 

  cat <<END
This script runs a target from a specified destination, after copying all its dependencies.

Use as:

  bazel run $TARGET -- [-r rsync options]... [-s ssh options]... [-t target options]... [-o|-h] [DEST]...

Accepted options:

  DEST         List of dests to copy the target to. The target will then be
               run on the first dest specified, unless -o is used.

               The list of dests is MANDATORY, unless it is already specified
               in the rule in the BUILD.bazel file. In that case, if you pass any
               dest on the command line the list will REPLACE the one supplied
               in the BUILD.bazel file.

               DEST can be:
                 1) A string without / or :, assumed to be a machine name.
		    In this case, ssh will be used to run the remote command,
                    and the defined destination directory will be appended.
                 2) A string containing / or :, assumed to be a full path
                    specified by the user. No path will be appended. ssh will
                    be used if there's a ':' in the name.

                 This roughly matches rsync and scp syntax: a string like
                 'machine00.corp' will be turned into 'machine00.corp:dest/path/',
                 and use ssh. A string like 'machine00.corp:whatever' is assumed
                 to already contain a path, and will not be mangled. A string like
                 './tests' will be used as a plain destination directory.

  -o           Don't run the binary, only copy all the files to the remote dests.

  -r [value]   Adds one or more rsync options.

     For example: "-r'-v' -r'--show-progress'" will append the "-v --show-progress"
     options to rsync.

  -t [value]   Adds one or more options to your target.

     For example: "-t'--baremetal'" will run the specified target with the "--baremetal"
     option. Those options are highly dependant on the target.

  -h           Prints this astonishingly helpful message.

****************************************************************************************
IMPORTANT/DANGER/ACHTUNG: rsync will overwrite and delete files in the output directory.
         Make sure to never point this script to your home, or to a populated directory.
****************************************************************************************
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
  DESTS=("$@")
}

[ "${#DESTS[@]}" -ge 1 ] || {
  help "You must specify one or more DEST to copy the output to"
}

target={target}
include={include}
destdir={destdir}
destpath="$destdir/${target//[^a-zA-Z0-9_-]/_}"
executable={executable}
workspace={workspace}

printrun() { echo "+ $*"; "$@"; }
is_fullpath() { [[ "$1" =~ [/:] ]] || return 1; return 0; }
is_remote() {
  if [[ "$1" =~ [:] ]] || ! [[ "$1" =~ [/:] ]]; then 
    return 0
  fi
  return 1 
}

echo "Copying files..."
set -e
destrun="" # Actual location where the test will be run from.
for dest in "${DESTS[@]}"; do
  # Machine already specifies a path, or it's a local directory.
  # Don't mess with the path supplied by the user.
  if is_fullpath "$dest"; then
    printrun $RSYNC_CMD --files-from="$include" "${RSYNC_OPTS[@]}" .. "$dest"
    test -n "$destrun" || destrun="$dest/$workspace"
  else
    printrun $RSYNC_CMD --files-from="$include" "${RSYNC_OPTS[@]}" .. "$dest:$destpath/"
    test -n "$destrun" || destrun="$destpath/$workspace"
  fi
done

# TODO(cccontavalli): better escaping, will fix it once we have more tests.
command="cd $destrun; MACHINES='${DESTS[*]}' ./$executable ${TARGET_OPTS[*]}"
[ "$ONLY_COPY" != "true" ] || {
  echo "Copy only mode was requesting - not running any command"
  echo "Would have run:"
  echo "    $command" 
  exit 0
}

[ "$NO_EXECUTE" != "true" ] || {
  echo "Target $target is not executable - only copied"
  exit 0
}

target="${DESTS[0]}"
echo "Running '$command' on $target..."
if is_remote "$target"; then
  printrun $SSH_CMD "${SSH_OPTS[@]}" "$target" -- "$command"
else
  printrun "$command"
fi
