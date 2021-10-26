#!/bin/bash
set -e

# Log affected targets
# Requires:
# * Image with `bash`
# * argv[1]: Path to list of changed targets
# * argv[2]: Path to list of changed tests
#
# Example usage:
#  - name: gcr.io/cloud-builders/git
#    entrypoint: bash
#    args:
#      - -c
#      - infra/cloudbuild/helpers/log_affected_targets.sh /affected-targets/build.txt /affected-targets/test.txt
#    volumes:
#      - name: affected-targets
#        path: /affected-targets


readonly CHANGED_TARGETS_FILE="$1"
readonly CHANGED_TESTS_FILE="$2"

echo "Building affected targets:"
cat "${CHANGED_TARGETS_FILE}"
echo ""
echo "Running affected tests:"
cat "${CHANGED_TESTS_FILE}"