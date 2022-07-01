load("@bazel_skylib//lib:shell.bzl", "shell")

def _exec_test(ctx):
    # A test rule must return an executable file, cannot reuse
    # the executable returned by another rule.
    template = """#!/bin/bash
ARGS={argv}; ARGS+=("$@")
{exec} {script} "${{ARGS[@]}}"
"""
    script = ctx.actions.declare_file("{}_test.sh".format(ctx.attr.name))
    ctx.actions.write(script, template.format(exec = (ctx.attr.must_fail and "!") or "exec", argv = shell.array_literal(ctx.attr.argv), script = ctx.executable.dep.short_path))
    runfiles = ctx.runfiles(ctx.files.dep).merge(ctx.attr.dep[DefaultInfo].default_runfiles)
    return [DefaultInfo(files = depset(ctx.files.dep), runfiles = runfiles, executable = script)]

exec_test = rule(
    doc = """Turns an executable target into a test target.

This is so that any executable target can be used as a test, including in
test_suite(), with a specific set of parameters. But also so that test targets
can be broken out into separate executable and test phases.

This is convenient as when a target is marked as 'test = True', it must be
named _test, and is always run under a different environment (wrapped in a
test-setup.sh), which changes the terminal behavior, backgrounds tasks, and
affects the --run_under flag - which can make some targets very hard to
work with for debugging purposes.

Example:

1) Let's say you have a rule like:

    sh_binary(
        name = "compute-valid",
        srcs = [ ... ],
    )

2) You can turn it into a test by using:

    exec_test(
        name = "compute-valid-delta",
        dep = [":compute-valid"],
        args = ["-delta", "1799"],
    )

The target "compute-valid-delta" will be a test, that will succeed
if "compute-valid.sh -delta 1799" exits with status 0.
""",
    implementation = _exec_test,
    test = True,
    attrs = {
        "dep": attr.label(
            doc = "Executable target to be converted into a test target",
            mandatory = True,
            executable = True,
            cfg = "host",
        ),
        "argv": attr.string_list(
            doc = "Additional arguments to pass to the executable target",
        ),
        "must_fail": attr.bool(
            default = False,
            doc = "Set to true if a non zero status should be interpreted as success",
        ),
    },
)
