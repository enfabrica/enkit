#!/bin/bash
set -e

# Run a set of test targets defined by a targets file.
# Requires:
# * Image with build environment
# * argv[1] is a path to a file containing a list of targets to test.
#
# Example usage:
#  - name: gcr.io/devops-284019/developer_testing:scott_presubmit_test
#    entrypoint: bash
#    args:
#      - -c
#      - infra/cloudbuild/helpers/bazel_test.sh /affected-targets/test.txt
#    volumes:
#      - name: affected-targets
#        path: /affected-targets

# Path to file containing list of targets to test, one per line
readonly TARGETS_FILE="$1"

if [[ ! -s "${TARGETS_FILE}" ]]; then 
  echo "No targets to test; skipping"
  exit 0
fi

cat "${TARGETS_FILE}" | xargs bazel test