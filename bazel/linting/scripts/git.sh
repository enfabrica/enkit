#!/usr/bin/env bash
#github_sha=$GIT_SHA1
#github_sha=$(git rev-parse HEAD)
##if [[ -z $GIT_SHA1 ]]
##  github_sha=$(git rev-parse HEAD)
##fi
#echo "inide of script"
#echo $1
#prev=$PWD
#cd $1
#echo "inside ${PWD}"
#echo "$(git --no-pager rev-parse HEAD)"
#echo "$(git --no-pager diff)"
#echo "$(git version)"
#cd $prev
if [ -z $1 ]; then
  echo "must present status file"
  exit 1
fi

line=$(grep GIT_CHANGED_FILES $1 | cut -d " "  -f2-)
read -a arr <<< $line
for i in "${arr[@]}"
do
   echo $i
   # do whatever on $i
done
