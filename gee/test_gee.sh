#!/bin/bash
#
# A quick bash script that exercises some of gee's basic functionality.
# Requires human intervention.  At some point, replace with expect.

SCRIPTDIR="$(dirname "$(readlink -f "$0")")"
GEE="${SCRIPTDIR}/gee.py"
DEFAULT_GEERC="${SCRIPTDIR}/geerc.default"
GEERC="${SCRIPTDIR}/test.gee.rc"
GEEDIR="${HOME}/test_gee.$$"
function g() {
  printf ">>> gee.py --config %s" "${GEERC}"
  printf " %q" "$@"
  printf "\n"
  "${GEE}" --config "${GEERC}" "$@"
  RC=$?
  return $RC
}

perl -p -e \
  's(^gee_dir.*)(gee_dir = "'"${GEEDIR}"'");s(internal)(enkit)g' <"${DEFAULT_GEERC}" >"${GEERC}"

set -e

g init org-64667743@github.com:enfabrica/enkit.git

# this should fail:
if g mkbr test1; then
  echo "Missed expected failure!"
  exit 1
else
  echo "RC=$?: Expected failure."
fi

cd "${GEEDIR}/enkit/master"
g mkbr test1
cd "${GEEDIR}/enkit/test1"
ls

printf "1 2 3\n4 5 6\n7 8 9\n" > matrix.txt
g commit -a -m "added matrix.txt"
g mkbr test2
printf "1 2 3\n4 X 6\n7 8 9\n" > matrix.txt
g commit -a -m "matrix: changed to X"
cd "${GEEDIR}/enkit/test2"
printf "1 2 3\n4 Y 6\n7 8 9\n" > matrix.txt
g commit -a -m "matrix: changed to Y"
# encounter a merge conflict:
g up



