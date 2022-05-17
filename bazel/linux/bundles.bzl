load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "RuntimeBundleInfo", "RuntimeInfo")
load("//bazel/linux:utils.bzl", "expand_deps", "get_compatible")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:files.bzl", "files_to_dir")
load("@bazel_skylib//lib:shell.bzl", "shell")

def _add_attr_bundle(ctx, bundle, name):
    """Used by _vm_bundle, helper to parase its attributes"""
    abin = getattr(ctx.attr, name + "_bin")
    aargs = getattr(ctx.attr, name + "_args")
    acmds = getattr(ctx.attr, name + "_cmds")

    if not abin and not acmds:
        return

    info = dict(commands = acmds, args = aargs)
    if abin:
        di = abin[DefaultInfo]
        info["binary"] = di.files_to_run.executable
        info["runfiles"] = di.default_runfiles

    bundle[name] = RuntimeInfo(**info)

def _vm_bundle(ctx):
    bundle = {}
    _add_attr_bundle(ctx, bundle, "prepare")
    _add_attr_bundle(ctx, bundle, "run")
    _add_attr_bundle(ctx, bundle, "check")

    return RuntimeBundleInfo(**bundle)

vm_bundle = rule(
    doc = """\
Packages one or more binaries into a bundle suitable for use from an emulator.

This is only needed if you need to provide a specific argv, or to
supply a script to prepare the environment to run the VM, or to clean
up afterward.

As an example, let's say you need to create an "internal-test" bundle that 1)
runs some setup commands outside the VM, 2) runs a test binary in the VM, and
3) checks the result of the test at the end of the run.

You can set up the bundle like this:

    sh_binary(
        name = "prepare-environment-outside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    sh_binary(
        name = "test-binary-inside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    sh_binary(
        name = "check-results-outside-vm",
        srcs = [ ... a shell script ... ],
        data = [ ... a bunch of static files ...],
    )

    vm_bundle(
        name = "internal-test"
        prepare_cmds = [
            "cp /etc/hosts $OUTPUT_DIR",
        ],
        prepare_bin = ":prepare-environment-outside-vm",
        prepare_args = "--read --aggressive",

        run_bin = ":test-binary-inside-vm",
        
        check_cmds = [
            "jq $OUTPUT_DIR > $OUTPUT_DIR/file.json",
        ],
        check_bin = ":check-results-outside-vm",
        check_args = "--check $OUTPUT_DIR/file.json",
    )
""",
    implementation = _vm_bundle,
    attrs = {
        "prepare_cmds": attr.string_list(
            doc = "Shell commands to run OUTSIDE THE VM to prepare the environment (before prepare_bin - optional)",
        ),
        "prepare_bin": attr.label(
            doc = "Binary to run OUTSIDE THE VM to prepare the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "prepare_args": attr.string(
            doc = "Optional parameters to pass to the prepare_bin. Can use shell expansion.",
        ),
        "run_cmds": attr.string_list(
            doc = "Shell commands to run OUTSIDE THE VM to run the environment (before run_bin - optional)",
        ),
        "run_bin": attr.label(
            doc = "Binary to run OUTSIDE THE VM to run the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "run_args": attr.string(
            doc = "Optional parameters to pass to the run_bin. Can use shell expansion.",
        ),
        "check_cmds": attr.string_list(
            doc = "Shell commands to check OUTSIDE THE VM to check the environment (before check_bin - optional)",
        ),
        "check_bin": attr.label(
            doc = "Binary to check OUTSIDE THE VM to check the environment (optional)",
            executable = True,
            cfg = "exec",
        ),
        "check_args": attr.string(
            doc = "Optional parameters to pass to the check_bin. Can use shell expansion.",
        ),
    },
)

def _kunit_bundle(ctx):
    ki = ctx.attr.image[KernelImageInfo]
    mods = get_compatible(ctx, ki.arch, ki.package, ctx.attr.module)
    alldeps = expand_deps(ctx, mods, ctx.attr.depth)

    commands = [
        # modprobe does not work correctly without /sys
        "mount -t sysfs sysfs /sys",
    ]

    inputs = []
    for kmod in alldeps:
        commands += ["", "# module " + package(kmod.label)]
        if kmod.setup:
            commands += kmod.setup

        for mod in kmod.files:
            if mod.extension != "ko":
                continue
            commands.append("load " + mod.short_path)
            inputs.append(mod)

    init = ctx.actions.declare_file(ctx.attr.name + "-kunit.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_kunit,
        output = init,
        substitutions = {
            "{target}": package(ctx.label),
            "{message}": "KUNIT TESTS",
            "{commands}": "\n".join(commands),
        },
        is_executable = True,
    )

    check = ctx.actions.declare_file(ctx.attr.name + "-check.sh")
    ctx.actions.expand_template(
        template = ctx.file.template_check,
        output = check,
        substitutions = {
            "{target}": package(ctx.label),
            "{parser}": ctx.executable._parser.short_path,
        },
        is_executable = True,
    )
    outside_runfiles = ctx.runfiles(files = ctx.attr._parser.files.to_list())
    outside_runfiles = outside_runfiles.merge(ctx.attr._parser.default_runfiles)
    inside_runfiles = ctx.runfiles(inputs)

    return [
        DefaultInfo(files = depset([init, check]), runfiles = inside_runfiles.merge(outside_runfiles)),
        RuntimeBundleInfo(
            run = RuntimeInfo(binary = init, runfiles = inside_runfiles),
            check = RuntimeInfo(binary = check, runfiles = outside_runfiles),
        ),
    ]

kunit_bundle = rule(
    doc = """\
Generates a directory containing the kernel modules, all their dependencies,
and an init script to run them as a kunit test.""",
    implementation = _kunit_bundle,
    attrs = {
        "module": attr.label(
            mandatory = True,
            providers = [KernelBundleInfo],
            doc = "The label of the KUnit linux kernel module to be used for testing. It must define a kunit_test_suite so that when loaded, KUnit will start executing its tests.",
        ),
        "image": attr.label(
            mandatory = True,
            providers = [KernelImageInfo],
            doc = "The label of a kernel image this test will run against. Important to select the correct architecture and package module.",
        ),
        "depth": attr.int(
            default = 5,
            doc = "Maximum recursive depth when expanding a list of kernel module dependencies.",
        ),
        "template_kunit": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/kunit.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "template_check": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/check_kunit.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_parser": attr.label(
            default = Label("//bazel/linux/kunit:kunit_zip"),
            doc = "KUnit TAP output parser.",
            executable = True,
            cfg = "host",
        ),
    },
)
