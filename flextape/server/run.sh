#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:f2de37b84cfdb699442ac4f1013a4400bd0f8a01be2d0b9fd28904cb7a125377"
