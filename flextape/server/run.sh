#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:a3af8d3ddf84d666a064eb4826d2aa8bb8dba42c9ddc1734192a0c8523f64ad4"
