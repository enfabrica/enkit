#!/usr/bin/bash
#
# Aliases for use with gee.
#
# Source this file from your .bashrc file:
#
#   source ~/gee/enkit/master/scripts/gee_aliases.bash

function gcd() {
  if (( "$#" == 0 )); then
    cat <<'EOT'
Usage: gcd <branch-name>

"gcd" changes the current working directory to the same root-relative directory
in another branch.

For example:

  cd ~/gee/enkit/branch1/foo/bar
  # now in ~/gee/enkit/branch1/foo/bar
  gcd branch2
  # now in ~/gee/enkit/branch2/foo/bar

EOT
    return 1
  fi

  local D="$(gee gcd "$@")"
  if [[ -n "${D}" ]]; then
    cd "${D}"
  fi
}
