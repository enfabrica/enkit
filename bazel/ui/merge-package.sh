#!/bin/bash
args=("$@")
counter=0
ss=""
while [ "$1" != "" ]; do
  if [ -z $ss ]; then
    ss=".[0]"
  else
    ss="$ss * .[$counter]"
  fi
  counter=$((counter+1))
  shift
done
echo "$(jq -s "$ss" "${args[@]}")"
