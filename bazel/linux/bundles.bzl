load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "RuntimeBundleInfo", "RuntimeInfo")
load("//bazel/linux:utils.bzl", "expand_deps", "get_compatible")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:files.bzl", "files_to_dir")
load("@bazel_skylib//lib:shell.bzl", "shell")

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
        template = ctx.file._template_kunit,
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
        template = ctx.file._template_check,
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
        "_template_kunit": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/kunit.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_template_check": attr.label(
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
