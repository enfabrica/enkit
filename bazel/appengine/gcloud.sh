#!/bin/sh

mytmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t 'gcloud-push')
cp -r {ropath}/src ${mytmpdir}
chmod -R ug+w ${mytmpdir}/src

echo {binary} {args}
{binary} {args} -path="${mytmpdir}"