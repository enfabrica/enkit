#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:17b71d9260fddbaa1f730c66ef7242338815e2a234bec488932d29e48e0f6431"
