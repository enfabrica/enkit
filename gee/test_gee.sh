#!/bin/bash
#
# A quick bash script that exercises some of gee's basic functionality.
# Requires human intervention.  At some point, replace with expect.

SCRIPTDIR="$(dirname "$(readlink -f "$0")")"
GEE="${SCRIPTDIR}/gee.py"
DEFAULT_GEERC="${SCRIPTDIR}/geerc.default"
GEERC="${SCRIPTDIR}/test.gee.rc"
GEEDIR="${HOME}/test_gee.foo"
function g() {
  printf "\n\n>>> gee.py --config %s" "${GEERC}"
  printf " %q" "$@"
  printf "\n"
  "${GEE}" --config "${GEERC}" "$@"
  RC=$?
  return $RC
}

perl -p -e \
  's(^gee_dir.*)(gee_dir = "'"${GEEDIR}"'");s(internal)(enkit)g' <"${DEFAULT_GEERC}" >"${GEERC}"

set -e

# clean up prior runs
rm -rf "${HOME}"/test_gee.*

g init org-64667743@github.com:enfabrica/enkit.git

# this should fail:
if g mkbr test1; then
  echo "Missed expected failure!"
  exit 1
else
  echo "RC=$?: Expected failure."
fi

cd "${GEEDIR}/enkit/master"

function test_merge_conflict() {
  echo ""
  echo "#############################################"
  echo "## testing merge conflict: $1"
  echo "#############################################"
  cd "${GEEDIR}/enkit/master"
  if [[ ! -d "${GEEDIR}/enkit/test1" ]]; then
    yes n | g mkbr test1
  fi
  cd "${GEEDIR}/enkit/test1"
  git reset --hard master
  printf "1 2 3\n4 5 6\n7 8 9\n" > matrix.txt
  g commit -f -a -m "added matrix.txt"
  if [[ ! -d "${GEEDIR}/enkit/test2" ]]; then
    yes n | g mkbr test2
  fi
  cd "${GEEDIR}/enkit/test2"
  git reset --hard test1
  cd "${GEEDIR}/enkit/test1"
  printf "1 2 3\n4 X 6\n7 8 9\n" > matrix.txt
  g commit -f -a -m "matrix: changed to X"
  cd "${GEEDIR}/enkit/test2"
  printf "1 2 3\n4 Y 6\n7 8 9\n" > matrix.txt
  g commit -f -a -m "matrix: changed to Y"
  # encounter a merge conflict:
  if [[ -n "$1" ]]; then
    yes $1 | g up
  else
    g up
  fi
}

test_merge_conflict t  # "theirs"
test_merge_conflict y  # "yours"
test_merge_conflict a  # "abort"
test_merge_conflict k  # "skip"



