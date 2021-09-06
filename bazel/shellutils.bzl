# Bazel rules for helping with shell scripts.

def sh_escape(x):
  # TODO(jonathan): improve this function if repr falls short.
  return repr(x)

EXEC_TEST_TEMPLATE= """
#!/usr/bin/env bash
set -e
"{command}" {args} {srcs}
"""

def _exec_test_impl(ctx):
  # Generic test that runs an executable.  Right now, this is only
  # used for bats_test, but this might be used in the future for
  # shellcheck_test and others.
  runfiles = ctx.runfiles(
      files = ctx.files.srcs + ctx.files.deps,
      collect_data = True,
  )
  runfiles = runfiles.merge(ctx.attr._command.default_runfiles)
  srcs = [f.short_path for f in ctx.files.srcs]
  script = EXEC_TEST_TEMPLATE.format(
          command = ctx.executable._command.short_path,
          args = " ".join([sh_escape(x) for x in ctx.attr.extra_args]),
          srcs = " ".join([sh_escape(x) for x in srcs]),
  )
  ctx.actions.write(
      output = ctx.outputs.executable,
      is_executable = True,
      content = script,
  )
  return DefaultInfo(
      runfiles = runfiles,
  )

bats_test = rule(
    attrs = {
        "srcs": attr.label_list(
            allow_files = [".bats"],
            doc="\"bats\" tests to run.",
        ),
        "extra_args": attr.string_list(
            doc="Extra arguments to pass to the command.",
        ),
        "deps": attr.label_list(
            doc="Extra dependencies to make available when test runs.",
        ),
        "_command": attr.label(
            default = Label("@bats_core//:bats"),
            executable = True,
            cfg = "host",
        ),
    },
    test = True,
    implementation = _exec_test_impl,
)
