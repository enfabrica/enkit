#!/bin/bash
# This script is a light wrapper around jq. It is designed to merge multiple json files together
args=("$@")
counter=0
ss=""
jq_path=$1
shift
while [ "$1" != "" ]; do
  if [ -z "$ss" ]; then
    ss=".[0]"
  else
    ss="$ss * .[$counter]"
  fi
  counter=$((counter+1))
  shift
done
$jq_path -s "$ss" "${args[@]:1}"
