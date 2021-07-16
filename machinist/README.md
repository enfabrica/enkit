Welcome to Machinist
--- 
##### What is machinist?
Machinist is a bare metal runner and manager,
It covers ssh authorization, remote execution and monitoring.

It has a hard dependency on enkit authorization server.
 
##### Code Layout:
```
client/ <- features and commands designed to be executed from an external users machine
cmd/ <- entrypoint to main package
config/ <- static configuration models
machine/ <- servers and handlers intended to run on bare metal 
mserver/ <- controlplane for machinist. Intended to run containerized
polling/ <- business logic for long running processes
rpc/ <- grpc proto files for communication
state/ <- mutable state models and features
testing/ <- e2e and integration tests
```

##### Features
- DNS server baked in for enrolled machines
- Exports prometheus metrics
- SSH Authorization through ssh certificates
- Installation through systemd
