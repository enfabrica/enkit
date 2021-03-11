#!/usr/bin/env bash

if [ -z $1 ]; then
  echo "must present status file"
  exit 1
fi

line=$(grep GIT_CHANGED_FILES $1 | cut -d " "  -f2-)
read -a arr <<< $line
for i in "${arr[@]}"
do
   echo $i
done
