# Enkit WORKSPACE initialization

WORKSPACE files are hard. In order to minimize the chance of errors and maintain
the same dependencies in enkit and its downstream dependencies, we'd like the
contents of enkit's WORKSPACE to be a repository macro that both enkit and
dependencies can execute. However, this is confounded by a few factors:

* Repositories can define macros that pull in other repositories; this can be
  repeatedly chained. This requires a sequence of repository rule declaration
  followed by load from that repository rule follwed by load, declaration, load,
  etc.
* Such a sequence is permitted in WORKSPACE files but not in .bzl files, where a
  unified repository macro would be defined. bzl files must have all loads at
  the top of the file as the first statements, not inside a function body or
  elsewhere in the file.
* A repository must be declared before a transitive load statement references
  it. This means that if `load("@foo//:some_file.bzl", "some_fn")` exists, `foo`
  must be already declared. Additionally, it means that `foo` must be declared
  if any `load()` encountered has that load statement, transitively.

This directory contains Starlark with the entire WORKSPACE logic, split into
stages to accomodate the above. Stage 1 contains just repository rule
declarations. Stage 2 contains elements that depend on stage 1, which are
largely macros that pull in transitive dependencies from repositories declared
in stage 1, as well as language-specific dependencies (Python pip packages,
golang packages, etc.) that depend on rules repositories declared in stage 1.
Stage 3 contains elements that depend on stage 2, including loads that
transitively reference repositories that only exist after stage 2 completes.

More stages may be added, if we add something that depends on stage 3 completing
to run successfully.