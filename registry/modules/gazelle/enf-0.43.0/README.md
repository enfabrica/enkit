There are bunch of patches we need to apply for golang packages.
But `go_deps.module_override` only allowed in root module. 
The same time we share modules between `enkit` and `internal` repos, both are root modules and `internal` depend on `enkit`.
The way to fix this is to have patches store in `gazelle` module itself and applied as default patches by injecting them [here](https://github.com/bazel-contrib/bazel-gazelle/blob/master/internal/bzlmod/go_deps.bzl#L160)