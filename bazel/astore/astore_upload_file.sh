#!/bin/sh

set -e

FILE="{file}"
TARGETS=( {targets} )
UIDFILE="{uidfile}"

TEMPTOML="$(mktemp /tmp/astore.XXXXX.toml)" || exit 1
trap 'rm -f "${TEMPTOML}"' EXIT

function update_build_file() {
  local UIDFILE="$1"
  local TARGET="$2"
  local FILE_UID="$3"

  if [[ ! -f "${UIDFILE}" ]]; then
    echo >&2 "Error: ${UIDFILE}: file not found"
    exit 3
  fi
  UIDFILE="$(readlink -f "${UIDFILE}")"
  local VARNAME="UID_$(basename "${TARGET}" | tr a-z A-Z | tr -c A-Z0-9\\r\\n _ )"
  local SEDSCRIPT="s/^${VARNAME} = \".*\"/${VARNAME} = \"${FILE_UID}\"/"
  if ! sed -i "${SEDSCRIPT}" "${UIDFILE}"; then
    echo >&2 "Error: sed script failed: ${SEDSCRIPT}"
    exit 4
  fi
  if ! grep "^${VARNAME} = \"${FILE_UID}\"" "${UIDFILE}" >&2 ; then
    echo >&2 "Error: failed to update ${VARNAME} in ${UIDFILE}"
    echo >&2 "       Is this variable missing from this file?"
    exit 5
  fi
  echo >&2 "Updated ${VARNAME} in ${UIDFILE}"
}

# astore doesn't tell us which metadata entry corresponds to which target, so
# we work around the issue by uploading the targets sequentially:
for TARGET in "${TARGETS[@]}"; do
  {astore} upload -G -f "${FILE}" "${TARGET}" -m "${TEMPTOML}"
  FILE_UID="$(grep -E "^  Uid = " "${TEMPTOML}" | awk '{print $3}' | tr -d \")"
  if [[ -z "${FILE_UID}" ]]; then
    echo >&2 "Error: no UID found for ${TARGET} uploaded as ${FILE}".
    exit 2
  fi
  echo >&2 "${TARGET} uploaded as ${FILE}: assigned UID ${FILE_UID}"
  if [[ -n "${UIDFILE}"  ]]; then
    update_build_file "${UIDFILE}" "${TARGET}" "${FILE_UID}"
  fi
done
