#!/usr/bin/env bash

# Sample script of how to start enkit built with bazel
# weth Delve debugger attached. Related vscode configuration
# Example `launch.json` to integrate with this script:
# {
#     "version": "0.2.0",
#     "configurations": [
#         {
#             "name": "connect to dlv",
#             "type": "go",
#             "debugAdapter": "dlv-dap", // `legacy` by default
#             "request": "attach",
#             "mode": "remote",
#             "port": 50034,
#             "host": "127.0.0.1", // can skip for localhost
#             "logOutput": "dap",
#             "showLog": true,
#             "trace": "verbose",
#             "substitutePath": [
#               { "from": "${workspaceFolder}", "to": "" }
#           ]
#         }
#     ]
# }


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
    astore \
    download \
    --force-uid \
    gbcsgrbqpptvz3ewn74b6e85umtsdicf \
    --output \
    /tmp/userspace-rcu-latest-0.15.tar.bz2 \
    --overwrite


    # bazel \
    # --loglevel-console debug \
    # affected-targets \
    # -s master \
    # -e gleb-INFRA-11538-experiments \
    # list \
    # --start_output_base ~/sob \
    # --start_workspace_log "${HOME}/sob_workspace_events.pb" \
    # --end_output_base ~/eob \
    # --end_workspace_log "${HOME}/eob_workspace_events.pb" \
    # --repo_root "${HOME}/develop/internal" \
    # --query 'deps(//...)'