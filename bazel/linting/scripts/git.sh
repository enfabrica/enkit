#!/usr/bin/env bash
if [ -z "$1" ]; then
  echo "must present status file"
  exit 1
fi
line=$(grep STABLE_GIT_MASTER_DIFF "$1" | cut -d " "  -f2-)
# shellcheck disable=SC2162
read -a arr <<< "$line"
for i in "${arr[@]}"
do
   echo "$i"
done
