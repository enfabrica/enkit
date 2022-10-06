#!/bin/bash
#
# This script captures the steps to manually test the special functionality
# in the //bazel/astore:defs.bzl rules.

set -e
trap 'printf "%s:%s: command %q failed with rc=%d\n" "${BASH_SOURCE}" "${LINENO}" "${BASH_COMMAND}" "$?"' ERR

echo "## Uploading to astore."
bazel run //bazel/astore/tests:test_astore_upload_file

echo "## Validate that BUILD file changes look right:"
git diff BUILD.bazel

echo "## These builds should all pass:"
bazel build \
  //bazel/astore/tests:test_astore_download_file \
  //bazel/astore/tests:test_astore_download_file_by_uid \
  //bazel/astore/tests:test_astore_download_file_with_digest \
  //bazel/astore/tests:test_astore_download_file_v1

echo "## Testing completed successfully."
trap - ERR
