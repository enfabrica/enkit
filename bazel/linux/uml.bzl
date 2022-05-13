load("//bazel/linux:runner.bzl", "CREATE_RUNNER_ATTRS", "create_runner")

def _kernel_uml_run(ctx):
    code = """
# The Bazel TEST_TMPDIR path tends to get long depending on the project
# structure and the current working directory at the time of test invocation.
# This overflows UNIX_PATH_MAX that UML sees when firing up. To prevent this,
# create a temporary directory and link it under the Bazel tree so it gets
# cleaned up properly after the test.
UML_DIR=$(mktemp -u)
ln -sf "$TMPDIR" "$UML_DIR"

OPTIONS=("uml_dir=$UML_DIR")
if [ -n "$ROOTFS" ]; then
  OPTIONS+=("ubd0=$ROOTFS" "hostfs=$RUNTIME")
else
  OPTIONS+=("rootfstype=hostfs" "init=$INIT")
fi

# If debugging is enabled, throw the user in a shell.
if [ -z "$INTERACTIVE" ]; then
  OPTIONS=("con0=null,fd:1" "con1=null,fd:1" "${{OPTIONS[@]}}")
else
  OPTIONS=("con0=fd:0,fd:1" "${{OPTIONS[@]}}" "init=/bin/sh")
fi
OPTIONS+=("${{EMULATOR_OPTS[@]}}")
OPTIONS+=("${{KERNEL_OPTS[@]}}")

echo 1>&2 '$' "$KERNEL" "${{OPTIONS[@]}}"
if [ -z "$INTERACTIVE" ]; then
  "$KERNEL" "${{OPTIONS[@]}}" | tee "$OUTPUT_FILE"
else
  "$KERNEL" "${{OPTIONS[@]}}"
  stty sane
fi
"""
    return create_runner(ctx, ["um"], code)

kernel_uml_run = rule(
    doc = """Runs code in an uml instance.

The code to run is specified by using the "runner" attribute, which
pretty much provides a self contained directory with an init script.
See the RuntimeBundleInfo provider for details.
""",
    implementation = _kernel_uml_run,
    attrs = CREATE_RUNNER_ATTRS,
    executable = True,
)
