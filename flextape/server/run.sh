#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:a044c5e5db4a06c95934ad5e6e62819c6461c659f273919d54433f96391405f4"