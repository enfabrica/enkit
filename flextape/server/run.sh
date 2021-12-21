#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:b75a18e00284c5dfe09096a37ecc4b873636e34fa14bff9a7e0e53e7971bbabf"