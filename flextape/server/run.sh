#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:c6b9c7bdc26a84ee2191a1a73cbe50bf5b76713411d5af486e3faac1939a6d29"
