load("//bazel/linux:uml.bzl", "kernel_uml_run")
load("//bazel/linux:qemu.bzl", "kernel_qemu_run")
load("//bazel/utils:macro.bzl", "mconfig", "mcreate_rule")
load("//bazel/utils:exec_test.bzl", "exec_test")
load("//bazel/linux:bundles.bzl", "kunit_bundle")
load("//bazel/linux:runner.bzl", "commands_and_runtime", "get_prepare_run_check")
load("//bazel/linux:providers.bzl", "RuntimeBundleInfo", "RuntimeInfo")
load("@bazel_skylib//lib:shell.bzl", "shell")

def _test_runner(ctx):
    script_begin = r"""#!/bin/bash
set -eu -o pipefail
GREEN=$(echo -en "\033[32m")
RED=$(echo -en "\033[31m")
RESET=$(echo -en "\033[0m")
failed_outputs=()
junit=""
tests=0
failures=0
run_test() {
    local title="$1"
    local script="$2"
    shift 2
    local args=("$@")
    local failed=1
    local output
    local status
    local start_time_ns
    local stop_time_ns
    local duration_ns
    local duration_s
    local error_msg=""

    printf "%-73s" "${title}"
    start_time_ns=$(($(date +%s%N)))

    output=$("${script}" "${args[@]}" 2>&1) && failed=0

    tests=$[tests + 1]
    if [ $failed -eq 0 ]; then
        status="${GREEN}PASSED${RESET}"
    else
        failures=$[failures + 1]
        status="${RED}FAILED${RESET}"
        failed_outputs+="==================== Test output for ${title}:\n${output}\n"
        error_msg="<failure message=\"failed\"/>"
    fi
    stop_time_ns=$(($(date +%s%N)))
    duration_ns=$(($stop_time_ns - $start_time_ns))
    duration_s=$(echo "$duration_ns/10^9" | bc -l)
    printf "%s in %.1fs\n" "$status" "$duration_s"
    junit+="
        <testsuite name=\"${title}\" tests=\"1\" failures=\"${failed}\">
            <testcase name=\"${title}\" status=\"run\" duration=\"${duration_s}\" time=\"${duration_s}\">${error_msg}</testcase>
        </testsuite>
"
}
"""

    script_end = r"""
for o in "${failed_outputs[@]}"; do
    echo -ne "$o"
done
if [ $failures -ne 0 ]; then
    echo "================================================================================"
fi
echo "Executed ${tests} test(s): $[tests - failures] test(s) pass(es) and ${failures} fail(s)."

echo '<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
'"${junit}"'
</testsuites>
' > "/tmp/output_dir/junit.xml"

exit $failures
"""

    prepares, runs, checks = get_prepare_run_check(ctx, ctx.attr.tests)

    outside_runfiles = ctx.runfiles()
    cprepares, outside_runfiles, _ = commands_and_runtime(ctx, "prepare", prepares, outside_runfiles, verbose = False)
    cchecks, outside_runfiles, _ = commands_and_runtime(ctx, "check", checks, outside_runfiles, verbose = False)
    cruns, inside_runfiles, run_labels = commands_and_runtime(ctx, "run", runs, ctx.runfiles(), verbose = False)

    tests = []
    for label, crun in zip(run_labels, cruns):
        tests.append("run_test {title} {cmd}".format(
            title = shell.quote(label),
            cmd = crun,
        ))

    script = ctx.actions.declare_file("{}_test_runner.sh".format(ctx.attr.name))
    ctx.actions.write(script, script_begin + "\n".join(tests) + script_end)
    return [RuntimeBundleInfo(
        prepare=RuntimeInfo(commands = cprepares, runfiles = outside_runfiles),
        run=RuntimeInfo(binary = script, runfiles = inside_runfiles),
        check=RuntimeInfo(commands = cchecks, runfiles = outside_runfiles),
    )]

test_runner = rule(
    doc = "Creates a test runner script that will execute a series of tests in the emulator, print the results and generate a junit.xml.",
    implementation = _test_runner,
    attrs = {
        "tests": attr.label_list(
            mandatory = True,
            doc = "List of executable targets to run in the emulator as tests. The duration and the exit status of each test is recorded and reported.",
        ),
    },
)

def qemu_test(name, kernel_image, setup, run, qemu_binary = None,
              config = {}, **kwargs):
    """Instantiates all the rules necessary to create a qemu based test.

    Specifically:
        {name}-test-runner: which creates a test runner script that will
           execute the tests in the emulator.
        {name}-run: which when run will execute a kernel_qemu_run target
           with the configs specified in config.
        {name}: which when executed as a test will invoke {name}-run and
           succeed if the target exits with 0.

    Args:
        kernel_image, run, qemu_binary: passed as is to the generated
            kernel_qemu_run rule. Exposed externally for convenience.
        config: dict, all additional attributes to pass to the
            kernel_qemu_run rule, generally created with mconfig().
    """

    runner_script = mcreate_rule(name, test_runner, "test-runner", [],
        kwargs, mconfig(tests = run),
    )
    runner = mcreate_rule(
        name, kernel_qemu_run, "run", config, kwargs,
        kwargs, mconfig(
            kernel_image = kernel_image,
            run = setup + [runner_script],
            qemu_binary = qemu_binary
        ),
    )
    exec_test(name = name, dep = runner, **kwargs)

def kunit_test(name, kernel_image, module, rootfs_image = None,
               kunit_bundle_cfg = {}, runner_cfg = {},
               runner = kernel_uml_run, **kwargs):
    """Instantiates all the rules necessary to create a kunit test.

    Creates 3 rules:
       {name}-runtime: which when built will create a kunit bundle for use.
       {name}-emulator: which when run will invoke the specified emulator
           together with the generated kunit runtime.
       {name}: which when executed as a test will invoke the emulator, and
           fail/succeed based on the results of the checks.
    Args:
      kernel_image: label, something like @type-of-kernel//:image,
          a kernel image to use.
      module: label, a module representing a kunit test to run.
      rootfs_image: optional, label, a rootfs image to use for the test.
      kunit_bundle_cfg: optional, dict, attributes to pass to the instantiated
          kunit_bundle rule, follows the mconfig use pattern.
      runner_cfg: optional, dict, attributes to pass to the instantiated
          runner rule, follows the mconfig use pattern.
      runner: a rule function, will be invoked to create the runner using
          the generated kunit bundle.
      kwargs: options common to all instantiated rules.
    """ 
    runtime = mcreate_rule(
        name,
        kunit_bundle,
        "runtime",
        kunit_bundle_cfg,
        kwargs,
        mconfig(module = module, image = kernel_image),
    )

    cfg = mconfig(run = [runtime], kernel_image = kernel_image)
    if rootfs_image:
        cfg = mconfig(cfg, rootfs_image = rootfs_image)
    name_runner = mcreate_rule(name, runner, "emulator", cfg, kwargs, runner_cfg)
    exec_test(name = name, dep = name_runner, **kwargs)
