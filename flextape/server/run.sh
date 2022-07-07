#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:0c4c11fc5d74f74925ea240471cdcb0b1a592d985ccc0d7094bbdf62e5460eb0"
