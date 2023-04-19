load("//bazel/linux:uml.bzl", "kernel_uml_run")
load("//bazel/linux:qemu.bzl", "kernel_qemu_run")
load("//bazel/utils:macro.bzl", "mconfig", "mcreate_rule")
load("//bazel/utils:messaging.bzl", "package")
load("//bazel/utils:exec_test.bzl", "exec_test")
load("//bazel/linux:bundles.bzl", "kunit_bundle", "vm_bundle")
load("//bazel/linux:runner.bzl", "get_prepare_run_check", "commands_and_runtime")
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

    # Print stdout and stderr to the terminal and capture it into a variable.
    # That way, bazel run shows the output in real time and bazel test can show
    # the output of only the failed commands at the end of the test.
    exec 5>&1
    output=$("${script}" "${args[@]}" 2>&1 | tee /dev/fd/5; exit ${PIPESTATUS[0]}) && failed=0

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
' > "$OUTPUT_DIR/junit.xml"

exit $failures
"""

    tests = []
    rtis = get_prepare_run_check(ctx, ctx.attr.tests)
    runfiles = ctx.runfiles()
    for index, torun in enumerate(rtis.run):
        testcommands, testrunfiles = commands_and_runtime(ctx, "test", [torun])

        tests.append("function test_{index}() {{( set -e\n{code}\n)}}".format(index = index, code = "\n".join(testcommands)))
        tests.append("run_test {title} test_{index}".format(
            index = index,
            title = shell.quote(package(torun.origin)),
        ))
        runfiles = runfiles.merge(testrunfiles)

    script = ctx.actions.declare_file("{}_test_runner.sh".format(ctx.attr.name))
    ctx.actions.write(script, script_begin + "\n".join(tests) + script_end)
    return [RuntimeBundleInfo(
        prepare = rtis.prepare,
        init = rtis.init,
        run = [RuntimeInfo(origin = ctx.label, binary = script, runfiles = runfiles)],
        cleanup = rtis.cleanup,
        check = rtis.check,
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

def qemu_test(
        name,
        kernel_image,
        setup,
        run,
        qemu_binary = None,
        config = {},
        bundle = {},
        **kwargs):
    """Instantiates all the rules necessary to create a qemu based test.

    Specifically:
        {name}-test-runner: which creates a test runner script that will
           execute the tests in the emulator.
        {name}-run: which when run will execute a kernel_qemu_run target
           with the configs specified in config.
        {name}-bundle: defining the init, setup, ... steps that are run
           within the VM.
        {name}: which when executed as a test will invoke {name}-run and
           succeed if the target exits with 0.

    Args:
        kernel_image, run, qemu_binary: passed as is to the generated
            kernel_qemu_run rule. Exposed externally for convenience.
        config: dict, all additional attributes to pass to the
            kernel_qemu_run rule, generally created with mconfig().
        bundle: dict, all additional parameters to pass to the
            bundle created implicitly by this macro.
    """

    # Do not pass test specific attributes to the created rules
    kwargs_copy = dict(kwargs)
    kwargs_copy.pop("size", None)
    kwargs_copy.pop("timeout", None)

    runner_bundle = mcreate_rule(
        name,
        vm_bundle,
        "bundle",
        bundle,
        kwargs_copy,
        mconfig(
            init = setup,
        ),
    )
    runner_script = mcreate_rule(
        name,
        test_runner,
        "test-runner",
        [],
        kwargs_copy,
        mconfig(tests = [runner_bundle] + run),
    )
    runner = mcreate_rule(
        name,
        kernel_qemu_run,
        "run",
        config,
        kwargs_copy,
        kwargs_copy,
        mconfig(
            kernel_image = kernel_image,
            run = [runner_script],
            qemu_binary = qemu_binary,
        ),
    )
    exec_test(name = name, dep = runner, **kwargs)

def kunit_test(
        name,
        kernel_image,
        module,
        rootfs_image = None,
        kunit_bundle_cfg = {},
        runner_cfg = {},
        runner = kernel_qemu_run,
        **kwargs):
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

    if runner == kernel_qemu_run:
        # printk timestamps breaks kunit result parsing in the QEMU runner
        cfg = mconfig(cfg, kernel_flags = ["printk.time=0"])

    name_runner = mcreate_rule(name, runner, "emulator", cfg, kwargs, runner_cfg)
    exec_test(name = name, dep = name_runner, **kwargs)
