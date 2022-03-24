#!/bin/bash

PORT=8000

docker run \
  -d \
  -t \
  -p ${PORT}:${PORT} \
  -e "PORT=${PORT}" \
  --name=flextape \
  --restart="always" \
  "gcr.io/devops-284019/infra/flextape@sha256:5f7ec2ff1e8aa34a0acd58691947cbc4ed7a9ceab06807e7d03e233dff1524dd"