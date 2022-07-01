#!/usr/bin/bash

FILE="$1"
CHECKSUM_WANT="$2"

find . -print
if [[ ! -f "${FILE}" ]]; then
  echo "File is missing: ${FILE}"
  exit 2
fi
CHECKSUM_GOT="$(/usr/bin/sum "${FILE}" | awk '{print $1}')"


if [[ "${CHECKSUM_WANT}" != "${CHECKSUM_GOT}" ]]; then
  echo "Checksums did not match!"
  echo "  Want: ${CHECKSUM_WANT}"
  echo "   Got: ${CHECKSUM_GOT}"
  exit 1
fi

echo "Checksum matches."
exit 0

