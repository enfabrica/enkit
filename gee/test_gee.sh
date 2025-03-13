#!/bin/bash

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




