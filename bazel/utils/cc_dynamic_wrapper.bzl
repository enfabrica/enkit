"""Rules to generate wrappers for dynmaic binaries."""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/utils:messaging.bzl", "fileowner")
load("//bazel/utils:messaging.bzl", "package")

def _cc_dynamic_wrapper_impl(ctx):
    template = """#!/bin/bash
export LD_LIBRARY_PATH={ldpaths}:$LD_LIBRARY_PATH
DEBUG=${{DEBUG:-{debug}}}
test -z "$DEBUG" || {{
  echo "===== dynamic wrapper {target} - files available ======="
  find -L .
  echo "===== running {run} $@ ======="
  echo "LD_LIBRARY_PATH=$LD_LIBRARY_PATH"
}}
exec {run} "$@"
"""
    run = ""
    targets = ctx.attr.deps
    if ctx.executable.bin:
        targets = [ctx.attr.bin] + targets
        run = shell.quote(ctx.executable.bin.short_path)
    if ctx.attr.run:
        run = ctx.expand_location(ctx.attr.run, targets = targets)

    dirs = {}
    runfiles = ctx.runfiles()
    for dep in targets:
        di = dep[DefaultInfo]

        files = dep.files.to_list()
        runfiles = runfiles.merge(ctx.runfiles(files = files))
        if di.files_to_run and di.files_to_run.executable:
            runfiles = runfiles.merge(ctx.runfiles(files = [di.files_to_run.executable]))

        deprf = di.default_runfiles
        if deprf and deprf.files:
            runfiles = runfiles.merge(deprf)
            files.extend(deprf.files.to_list())

        for f in files:
            # Why not use CcInfo or info provided through providers?
            #
            # Unfortunately, libraries and binaries can be brought in via
            # rules_foreign_cc or custom rules that either don't provide
            # the provider, or don't fill it enough to be useful.
            #
            # Both bazel native C/C++ rules and those rules, however,
            # typically do bring in all dependencies as runfiles or deps,
            # as the dependencies are needed at run time by the binary.
            #
            # This code exploits this by adding any .so file or any file
            # containing .so. as part of the name in the LD_LIBRARY_PATH.
            if not f.path.endswith(".so") and not ".so." in f.path:
                if ctx.attr.debug:
                    print("FILE", fileowner(f))
                continue

            if ctx.attr.debug:
                print("LIBRARY", fileowner(f))
            dirs[paths.dirname(f.short_path)] = True

    ctx.actions.write(ctx.outputs.out, template.format(
        debug = ctx.attr.debug or "",
        target = package(ctx.label),
        run = run,
        ldpaths = "\"${PWD}\"/" + ":\"${PWD}\"/".join([shell.quote(d) for d in dirs]),
    ), is_executable = True)

    return [DefaultInfo(executable = ctx.outputs.out, runfiles = runfiles)]

_cc_dynamic_wrapper = rule(
    doc = """Creates a wrapper to set LD_LIBRARY_PATH with all dependent .so libraries.

When using rules_foreign_cc or dynamic libraries, the generated binaries
are often not runnable outside of `bazel run`, or even within `bazel run`
without first setting LD_LIBRARY_PATH correctly so the linker can find
the dependent .so files.

This rules goes through all the dependencies of a generated binary and
creates a shell wrapper that sets LD_LIBRARY_PATH correctly before execing
the binary itself.

Generally, you would use this rule through its macro, cc_dynamic_wrapper.

In its simplest form, you can use it as:

    cc_dynamic_wrapper(
        name = "shell-wrapper", # an executable `shell-wrapper` is created.
        bin = ":label_of_your_cc_binary",
    )

Which will walk the dependencies of `:label_of_your_cc_binary`, look for
.so files and generate a `shell-wrapper` executable script with the proper
LD_LIBRARY_PATH environment variables set.

The rule has a few convenient features:

1. You can use the `deps` attribute to specify additional dependencies.
   Those dependencies will be carried on as runfiles for the generated
   shell script, and any library found there will be added to the
   LD_LIBRARY_PATH environment variable.

2. You can use the `run` attribute to override the command run by the
   wrapper. For example, by specifying a different binary to run, or
   by adding command line arguments. The `run` string is copied as
   is in the generated wrapper, with no escaping.

   For example, let's say you have a target built with `rules_foreign_cc`
   named "@rdma-core//:rdma-core", that includes **multiple** binaries,
   like `ib_send_lat` or `perftest`. You can use `cc_dynamic_wrapper`
   to create a single executable as:

       cc_dynamic_wrapper(
           name = "ib_send_lat",
           run = "./rdma-core/bin/ib_send_lat",
           deps = ["@rdma-core//:rdma-core"],
       )

3. You can generate a "pure wrapper", with no binary. For example, to
   allow you to run an arbitrary command (or a shell) with the correct
   environment. For example, if you define a target like:
   
       cc_dynamic_wrapper(
           name = "rdma-wrapper",
           deps = ["@rdma-core//:rdma-core"],
       )

   You can then run:

       bazel run :rdma-wrapper -- ls
       bazel run :rdma-wrapper -- /bin/bash
       bazel run :rdma-wrapper -- printenv

   ... and all of those will have the LD_LIBRARY_PATH set based on
   the content of the `@rdma-core//:rdma-core` dependency.

The generated wrapper has a few feature available at run time:

1. If you export the environment variable `DEBUG=true`, the script
   will print useful debug information. For example:

       DEBUG=true bazel run :rdma-wrapper -- ls

   will print all generated paths and command.

2. Extra arguments passed to bazel are propagated to the wrapper,
   so you can use `bazel run :wrapper-target -- /etc/hosts` for
   example.

IMPORTANT: this rule does not use CcInfo or other metadata carried by bazel
with the target. Instead, it looks for any .so file or any file containing
`.so.` as part of the name, and adds the corresponding path to LD_LIBRARY_PATH.
On one side, this allows the rule to work across binaries built natively with
cc_binary/cc_library rules, binaries built with rules_foreign_cc, or imported
binaries via filegroup() or cc_import - as long as the target brings in the .so
files it needs at run time. Beware that this can cause extra paths added to
LD_LIBRARY_PATH.
""",
    implementation = _cc_dynamic_wrapper_impl,
    executable = True,
    attrs = {
        "bin": attr.label(
            doc = "Label of a binary to run - if neither bin or run are specified, the wrapper will run $1 passed at run time.",
            executable = True,
            allow_files = True,
            cfg = "target",
        ),
        "run": attr.string(
            doc = """\
Command the wrapper should run - if specified, overrides bin. Can be an
arbitrary shell command with arguments, it is not escaped by the rule.
It is subject to location expansion, so it can contain $(location ...)
and similar patterns.""",
        ),
        "deps": attr.label_list(
            allow_files = True,
            doc = "Arbitrary deps to add to the runfiles. Any .so file found will cause the corresponding directory to be added to LD_LIBRARY_PATH.",
        ),
        "out": attr.output(mandatory = True, doc = "Name of the output shell wrapper"),
        "debug": attr.bool(
            doc = "If set to true, will print bazel debug information and mark DEBUG=true by default in the generated wrapper",
        ),
    },
)

def cc_dynamic_wrapper(name, *args, **kwargs):
    kwargs.setdefault("out", name)
    kwargs.setdefault("name", name)
    return _cc_dynamic_wrapper(*args, **kwargs)
