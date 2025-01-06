#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "us-docker.pkg.dev/enfabrica-container-images/infra-prod/flextape@sha256:25603b354ca910d70342ed1de3a6f984fe8442337c6019c8e0023c0a6ec866aa"
