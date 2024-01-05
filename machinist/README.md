# Machinist
Machinist is a combination of a server and client, where the client runs on each
machine on a local network, checking in with a single server instance on said
network.

## Responsibilities
Machinist is responsible for:
- Being a source-of-truth for actual machine health and networking configuration
  - This allows for good dashboards around what machines exist, when they were
    last seen, their IP in the face of DHCP/static config mishaps, etc.
  - Machinist server can advertise machines via DNS based on this info
  - Machinist server can advertise machines to scrape to metrics collectors
- Bootstrapping configuration management systems
  - Machinist client, being a static binary, is easily deployed where there is
    no config management client installed
  - Machinist client should not expand to be a full config management system,
    since this scope is very large and best covered by existing tools
  - Some amount of configuration is needed on a configuration agent itself (e.g.
    config management server URL, remote logging URL, etc.)

Currently, machinist is responsible for these deprecated actions:
- bootstrapping SSH configuration/access (should be managed by a config
  management system instead)
 
## Code Layout:
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
