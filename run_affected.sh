#!/usr/bin/env bash

set -ex

target=//enkit
bazel build --strip=never -c dbg "${target}"
outs=$(bazel cquery --strip=never -c dbg --output=files "${target}")
n=${#outs[@]}
if [[ "$n" -gt 1 ]]; then
  echo "too many outputs, not sure which one to debug (is this necessary?)"
fi
command="${PWD}/${outs[0]}"
"${command}" \
    bazel \
    --loglevel-console debug \
    affected-targets \
    -s master \
    -e gleb/ENGPROD-1075-migrate_astore_rules_to_repository_ctx_and_cred_helper \
    list \
    --start_output_base ~/sob \
    --end_output_base ~/eob \
    --start_workspace_log "${HOME}/sob_workspace_events.pb" \
    --end_workspace_log "${HOME}/eob_workspace_events.pb" \
    --repo_root "${HOME}/develop/internal" \
    --query "deps(//...)"

    # --query 'deps(//systest/src/lib:files)'

    # --query "@enf-ubuntu-impish-generic//:*"
    #--query "deps(//driver/core:enf-ubuntu-impish-generic-enf_core)"
    # --query "@enf-ubuntu-impish-generic//:*"
    # --query "deps(//driver/core:enf-ubuntu-impish-generic-enf_core)"
    # --query "allrdeps(@enf-ubuntu-impish-generic//:*)"
