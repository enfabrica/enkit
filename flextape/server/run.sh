#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:31dfe5ff6a646e80de374a8db484f83e3cc57b4c8bdc7670bb351de470ba0419"
