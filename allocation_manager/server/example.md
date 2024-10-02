# inside dev docker

# compile your own version
cd allocation_manager/server

bazel build :server && \

../../bazel-bin/allocation_manager/server/server_/server --service_config=allocation_manager_config.textproto >out.log

Send process to background by pressing ctrl-z followed by bg. Or start it in background with
the `&` added to the end of command.

Scrape metrics:

```sh
$ curl -s http://localhost:6435/metrics
...
# HELP allocation_manager_unit_operations_total Total number of operations performed on units
# TYPE allocation_manager_unit_operations_total counter
allocation_manager_unit_operations_total{operation="Expire",unit="a"} 23
allocation_manager_unit_operations_total{operation="Expire",unit="b"} 23
allocation_manager_unit_operations_total{operation="Expire",unit="back-to-back-nc-gpu-11-12"} 23
```

# just use it
cp allocation_manager_config.textproto /tmp
(cd /tmp && enkit astore download infra/allocation_manager/server)
/tmp/server --service_config=allocation_manager_config.textproto

# where do logs go?

# debug protos
pushd bazel-bin/allocation_manager/client/client.runfiles
env PYTHONPATH=com_google_protobuf/python:enfabrica:$(echo $(find . -name site-packages | fmt -1 ) | tr ' ' : ) python3
from allocation_manager.proto import allocation_manager_pb2 as apb

bazel build //allocation_manager/client/allocation_manager_client:allocation_manager_client && \
../../../bazel-bin/allocation_manager/client/allocation_manager_client/allocation_manager_client_/allocation_manager_client 127.0.0.1 6433 a topology.yaml sleep 30 --purpose=purposely
