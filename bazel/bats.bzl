# Run tests using bats
#
# Derived from https://github.com/filmil/bazel-bats

BATS_TEMPLATE = """
#!/usr/bin/env bash
set -e
export TMPDIR="${{TEST_TMPDIR}}"
export PATH="{deps_paths}:${{PATH}}"
"{bats}" --formatter tap {test_paths}
"""

def _bats_test_impl(ctx):
  runfiles = ctx.runfiles(
      files = ctx.files.srcs + ctx.files.deps,
      collect_data = True,
  )
  runfiles = runfiles.merge(ctx.attr._bats.default_runfiles)
  tests = [f.short_path for f in ctx.files.srcs]
  paths = [f.dirname for f in ctx.files.deps]
  deps_paths=":".join(paths)
  script = BATS_TEMPLATE.format(
          bats = ctx.executable._bats.short_path,
          deps_paths = deps_paths,
          test_paths = " ".join(["\"{}\"".format(x) for x in tests]),
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
            allow_files = True,
        ),
        "deps": attr.label_list(),
        "_bats": attr.label(
            default = Label("@bats_core//:bats"),
            executable = True,
            cfg = "host",
        ),
    },
    test = True,
    implementation = _bats_test_impl,
)
