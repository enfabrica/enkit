# Bazel rules for helping with shell scripts.

load("@bazel_skylib//lib:shell.bzl", "shell")

EXEC_TEST_TEMPLATE = """
#!/usr/bin/env bash
set -e
"{command}" {args} {srcs}
"""

def _external_exec_test_impl(ctx):
    # Generic test that runs an external executable.  Right now, this is only
    # used for shellcheck_test (shellcheck requires cabal to build, and so bazel
    # integration is non-trivial).
    runfiles = ctx.runfiles(
        files = ctx.files.srcs + ctx.files.deps,
        collect_data = True,
    )
    srcs = [f.short_path for f in ctx.files.srcs]
    script = EXEC_TEST_TEMPLATE.format(
        command = ctx.attr._command,
        args = " ".join([shell.quote(x) for x in ctx.attr.extra_args]),
        srcs = " ".join([shell.quote(x) for x in srcs]),
    )
    ctx.actions.write(
        output = ctx.outputs.executable,
        is_executable = True,
        content = script,
    )
    return DefaultInfo(
        runfiles = runfiles,
    )

shellcheck_test = rule(
    doc = """
      Runs shellcheck on a shell script.
    """,
    attrs = {
        "srcs": attr.label_list(
            allow_files = True,
            doc = "Shell scripts to check.",
        ),
        "extra_args": attr.string_list(
            doc = "Extra arguments to pass to shellcheck.",
        ),
        "deps": attr.label_list(
            doc = "Extra dependencies to make available when shellcheck runs.",
        ),
        "_command": attr.string(
            default = "/usr/bin/shellcheck",  # available in dev container.
            doc = "Path to external shellcheck command.",
        ),
    },
    test = True,
    implementation = _external_exec_test_impl,
)
