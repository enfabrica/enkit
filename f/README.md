# What the `f`?

This directory contains targets that are Bazel [user-defined build
settings](https://bazel.build/extending/config#user-defined-build-settings).
These settings are essentially "custom bazel flags" that can be specified on the
command-line to alter the behavior of particular rules.

Since these flags are referenced on the command-line as labels (like
`--//f/some:flag` or `--@enkit//f/some:flag`) the names can get quite long. In
an effort to shorten the names as much as possible, the flags are specified in
this top-level `f` directory, with subdirectories for different rule categories.