# inside dev docker

# compile your own version
cd allocation_manager/server

bazel build :allocation_manager && \

../../bazel-bin/allocation_manager/server/allocation_manager_/allocation_manager --service_config=allocation_manager_config.textproto

Send process to background by pressing ctrl-z followed by bg. Or start it in background with
the `&` added to the end of command.

Optionally, have example allocation code in `main`:

```go
	topo := apb.Topology{
		Name: "Unit Name 2", Config: "Unit Config",
	}
	a := service.NewUnit(topo)

	a.DoOperation("allocate")
	a.DoOperation("release")
	a.DoOperation("allocate")
	a.DoOperation("release")
	a.DoOperation("allocate")

```

Scrape metrics:

```sh
$ curl -s http://localhost:6435/metrics
....
# HELP unit_operations_total Total number of operations performed on units
# TYPE unit_operations_total counter
unit_operations_total{kind="service.unit",operation="allocate",unit="Unit Name 2"} 3
unit_operations_total{kind="service.unit",operation="release",unit="Unit Name 2"} 2
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
