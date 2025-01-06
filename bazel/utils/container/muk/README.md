# muk

`muk` is a tool that builds Docker images based on a structured-data definition,
to avoid Dockerfile duplication. This tool is necessary because no sane bazel
rules exist to build Docker images in such a non-hermetic way; yet, it is still
useful to do so. The tool will be integrated into bazel as an executable rule
(`bazel run` will perform all the building)

It will:
* download out-of-band dependencies (e.g. from astore)
* generate a Dockerfile
* copy the Dockerfile + deps into a tempdir
* build and optionally tag + push the image
