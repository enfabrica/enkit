#!/usr/bin/env bash

set -ex

target=//enkit
bazel build --strip=never -c dbg "${target}"
outs=$(bazel cquery --strip=never -c dbg --output=files "${target}")
n=${#outs[@]}
if [[ "$n" -gt 1 ]]; then
  echo "too many outputs, not sure which one to debug (is this necessary?)"
fi
command="${outs[0]}"
dlv exec "${command}" \
    --log \
    --log-output=dap \
    --headless \
    --listen=127.0.0.1:50034 \
    --api-version=2 \
    -- \
    bazel \
    --loglevel-console debug \
    affected-targets \
    -s master \
    -e gleb-INFRA-11538-experiments \
    list \
    --start_output_base ~/sob \
    --end_output_base ~/eob \
    --repo_root "${HOME}/develop/internal" \
    --query 'deps(//...)'
