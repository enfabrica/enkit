All embedded frontends live in this directory.

### Development workflow

1. Start the dev server of the app using the bazel target start
> bazel run //ui/ptunnel:start

2. Start the backend service(s) that the frontend needs.
> e.g. bazel run //proxy:enproxy

### Deploying workflow
At golang production build time, it should run the build target here and automatically add it as 
an embedded source in the golang binary which requires the frontend(s)
