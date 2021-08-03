#!/bin/bash

echo "here" >> "meow.txt"
echo "hello world"
ls
echo "$OUT"
if [[ "$STRATEGY" == "ALL" ]]; then
  sort -ub "$@" >> "$OUT"
fi


line=$(grep STABLE_GIT_MASTER_DIFF "$1" | cut -d " "  -f2-)
# shellcheck disable=SC2162
read -a arr <<< "$line"
for i in "${arr[@]}"
do
   echo "$i"
done

#
#line=$(grep STABLE_GIT_MASTER_DIFF "$1" | cut -d " "  -f2-)
#for i in "${changed_files[@]}"
#do
#  if [[ $i == *.go ]]; then
#    go_package=$(dirname $i)
#    $GOLANGCI_LINT run "$go_package" --verbose --issues-exit-code 0 2>&1 | sed 's/.*'enkit'//' | tee ${LINT_OUTPUT}
#  fi
#done