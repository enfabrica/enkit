#!/usr/bin/env bash

# Sample script of how to start enkit built with bazel
# weth Delve debugger attached. Related vscode configuration
# is in `launch.json`.

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
    --start_workspace_log "${HOME}/sob_workspace_events.pb" \
    --end_output_base ~/eob \
    --end_workspace_log "${HOME}/eob_workspace_events.pb" \
    --repo_root "${HOME}/develop/internal" \
    --query 'deps(//...)'
