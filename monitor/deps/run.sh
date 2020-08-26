docker run \
    -d \
    -e GF_AUTH_ANONYMOUS_ENABLED=true \
    -e GF_AUTH_ANONYMOUS_ORG_NAME=default \
    -e GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer \
    -v `pwd`/grafana:/var/lib/grafana \
    --net host grafana/grafana

#docker start -ai suspicious_sanderson

#    -v `pwd`/console_libraries:/usr/share/prometheus/console_libraries \
#    -v `pwd`/consoles:/usr/share/prometheus/consoles \

docker run \
    --net host \
    -v `pwd`/prometheus/config:/etc/prometheus \
    -v `pwd`/prometheus/data:/prometheus \
    prom/prometheus
