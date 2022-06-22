#!/bin/bash

cmp <(./bazel/utils/echo-argv_test_test.sh) - <<'EOF'
arg1
arg2 with space
arg3 with `characters`
EOF
test "$?" == 1 || {
    echo "first comparison was expected to fail - test not working?" 1>&2
    exit 1
}

set -eu -o pipefail

cmp <(./bazel/utils/echo-argv_test_test.sh) - <<'EOF'
arg1
arg2 with space
arg3 with $NASTY `characters`
EOF
