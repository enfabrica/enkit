load("//bazel/linux:runner.bzl", "CREATE_RUNNER_ATTRS", "create_runner")

def _kernel_uml_test(ctx):
    code = """
# The Bazel TEST_TMPDIR path tends to get long depending on the project
# structure and the current working directory at the time of test invocation.
# This overflows UNIX_PATH_MAX that UML sees when firing up. To prevent this,
# create a temporary directory and link it under the Bazel tree so it gets
# cleaned up properly after the test.
UML_DIR=$(mktemp -u)
ln -sf "$TMPDIR" "$UML_DIR"

OPTIONS=()
if [ -n "$ROOTFS" ]; then
  OPTIONS+=("ubd0=$ROOTFS" "hostfs=$RUNTIME")
else
  OPTIONS+=("rootfstype=hostfs" "init=$INIT")
fi

# If debugging is enabled, throw the user in a shell.
if [ -n "$INTERACTIVE" ]; then
  "$KERNEL" con=pty con0=fd:0,fd:1 uml_dir="$UML_DIR" "${{OPTIONS[@]}}" init=/bin/sh "$@" </dev/tty >/dev/tty || true
else
  "$KERNEL" con0=null,fd:1 con1=null,fd:1 uml_dir="$UML_DIR" "${{OPTIONS[@]}}" "$@" | tee "$OUTPUTFILE"
fi
"""
    return create_runner(ctx, ["um"], code)

kernel_uml_test = rule(
    doc = """Runs code in an uml instance.

The code to run is specified by using the "runner" attribute, which
pretty much provides a self contained directory with an init script.
See the RuntimePackageInfo provider for details.
""",
    implementation = _kernel_uml_test,
    attrs = CREATE_RUNNER_ATTRS,
    test = True,
)
