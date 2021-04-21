IMAGE="gcr.io/devops-284019/infra/flexlm:license-manager-server"
docker run -dt --name="license-manager" -p 8080:8080 --restart="on-failure" $IMAGE
