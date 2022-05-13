#!/bin/bash

test -r "$OUTPUT_FILE" || {
    echo 1>&2 "The OUTPUT_FILE environment variable is not set or points"
    echo 1>&2 "to a non existing file - checker run incorrectly?"
    exit 1
}

# See SF-73 for background
if grep -E -q '^1..0' "$OUTPUT_FILE" ; then
    # munge the test output to fix-up the number of test suites
    N_TESTS="$(grep -c "    # Subtest:" "$OUTPUT_FILE")" || true
    sed -i -e "s/^1..0/1..$N_TESTS/" "$OUTPUT_FILE" || true
fi

if [ "$N_TESTS" == "0" ] || ! python3 "{parser}" parse < "$OUTPUT_FILE"; then
  if [ "$N_TESTS" == "0" ]; then
    echo 1>&2 "=======> NO TESTS WERE RUN! Something went wrong."
  else
    echo 1>&2 "=======> TESTS FAILED. Scroll up to see the error."
  fi
  exit 100
fi
