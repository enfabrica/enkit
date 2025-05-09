#!/usr/bin/env bash

set -e
cat >/dev/null
TOKEN="$(gcloud auth --quiet print-access-token)"
jq -n --arg t "$TOKEN" '{headers:{"Authorization":[ "Bearer " + $t ]}}'
