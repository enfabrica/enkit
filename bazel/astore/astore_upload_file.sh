#!/usr/bin/env bash

set -e

FILE="{file}"
TARGETS=( {targets} )
UIDFILE="{uidfile}"
UPLOAD_TAG="{upload_tag}"
PRINT_UID="{print_uid_stdout}"

TEMPTOML="$(mktemp /tmp/astore.XXXXX.toml)" || exit 1
trap 'rm -f "${TEMPTOML}"' EXIT

function update_build_file() {
  local UIDFILE="$1"
  local TARGET="$2"
  local FILE_UID="$3"
  local FILE_SHA="$4"

  if [[ ! -f "${UIDFILE}" ]]; then
    echo >&2 "Error: ${UIDFILE}: file not found"
    exit 3
  fi
  UIDFILE="$(readlink -f "${UIDFILE}")"
  local VARNAME="$(basename "${TARGET}" | tr a-z A-Z | tr -c A-Z0-9\\r\\n _ )"
  local UID_VARNAME="UID_${VARNAME}"
  local SHA_VARNAME="SHA_${VARNAME}"
  local UID_SEDSCRIPT="s/^${UID_VARNAME} = \".*\"/${UID_VARNAME} = \"${FILE_UID}\"/"
  local SHA_SEDSCRIPT="s/^${SHA_VARNAME} = \".*\"/${SHA_VARNAME} = \"${FILE_SHA}\"/"
  if ! sed -i -e "${UID_SEDSCRIPT}" -e "${SHA_SEDSCRIPT}" "${UIDFILE}"; then
    echo >&2 "Error: sed command failed to execute script:"
    echo >&2 "  ${UID_SEDSCRIPT}"
    echo >&2 "  ${SHA_SEDSCRIPT}"
    exit 4
  fi
  if ! grep "^${UID_VARNAME} = \"${FILE_UID}\"" "${UIDFILE}" >&2 ; then
    echo >&2 "Error: failed to update ${UID_VARNAME} in ${UIDFILE}"
    echo >&2 "       Is this variable missing from this file?"
    exit 5
  fi
  echo >&2 "Updated ${UID_VARNAME} in ${UIDFILE}"
}

# astore doesn't tell us which metadata entry corresponds to which target, so
# we work around the issue by uploading the targets sequentially:
for TARGET in "${TARGETS[@]}"; do
  {astore} upload ${UPLOAD_TAG} -G -f "${FILE}" "${TARGET}" -m "${TEMPTOML}"
  FILE_UID="$(grep -E "^  Uid = " "${TEMPTOML}" | awk '{print $3}' | tr -d \")"
  FILE_SHA="$(sha256sum "${TARGET}" | awk '{print $1}')"
  if [[ -z "${FILE_UID}" ]]; then
    echo >&2 "Error: no UID found for ${TARGET} uploaded as ${FILE}".
    exit 2
  fi
  echo >&2 "${TARGET} uploaded as ${FILE}: assigned UID ${FILE_UID}"
  if [[ ${PRINT_UID:-false} == "true" ]]; then
    echo "${TARGET} uploaded as ${FILE}: assigned UID ${FILE_UID}"
  fi
  if [[ -n "${UIDFILE}"  ]]; then
    update_build_file "${UIDFILE}" "${TARGET}" "${FILE_UID}" "${FILE_SHA}"
  fi
done
