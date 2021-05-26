IMAGE="gcr.io/devops-284019/infra/flexlm:license-manager-server"
PORT=8585
docker run -dt --name="license-manager" -p $PORT:$PORT --restart="on-failure" $IMAGE $PORT
