#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:7ab742e551544c4d7e05f2fbc8c90a4654094a6ac0fe9fe3dbeb934dddaf8aa9"
