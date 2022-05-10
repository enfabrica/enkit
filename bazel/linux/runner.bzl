load("//bazel/linux:providers.bzl", "KernelImageInfo", "RootfsImageInfo", "RuntimePackageInfo")
load("//bazel/utils:messaging.bzl", "location", "package")
load("@bazel_skylib//lib:shell.bzl", "shell")

CREATE_RUNNER_ATTRS = {
    "kernel_image": attr.label(
        mandatory = True,
        providers = [DefaultInfo, KernelImageInfo],
        doc = "The kernel image that will be used to execute this test. A string like @stefano-s-favourite-kernel-image, referencing a kernel_image(name = 'stefano-s-favourite-kernel-image', ...",
    ),
    "rootfs_image": attr.label(
        mandatory = False,
        providers = [RootfsImageInfo],
        doc = """\
The rootfs image that will be used to execute this test.

A string like @stefano-s-favourite-rootfs-image, referencing a rootfs_image(name = 'stefano-s-favourite-rootfs-image', ...).
If not specified, the current root of the filesystem will be used as rootfs.
""",
    ),
    "runtime": attr.label(
        mandatory = True,
        providers = [RuntimePackageInfo],
        doc = "A target returning a RuntimePackageInfo, with commands to run in the emulator.",
    ),
    "_template": attr.label(
        allow_single_file = True,
        default = Label("//bazel/linux:templates/runner.template.sh"),
        doc = "The template to generate the bash script used to run the tests.",
    ),
}

def create_runner(ctx, archs, code, runfiles = None, extra = {}):
    ki = ctx.attr.kernel_image[KernelImageInfo]
    if archs and ki.arch not in archs:
        fail(
            location(ctx) + ("the kernel image '{name}' of architecture '{arch}' " +
                             "does not support the required architectures {archs}. Check 'arch = ...'").format(
                name = ki.name,
                arch = ki.arch,
                archs = archs,
            ),
        )

    inputs = depset(transitive = [
        ctx.attr.kernel_image.files,
        ctx.attr.runtime.files,
    ])

    rootfs = ""
    if ctx.attr.rootfs_image:
        rootfs = ctx.attr.rootfs_image[RootfsImageInfo].image.short_path
        inputs = depset(transitive = [inputs, ctx.attr.rootfs_image.files])

    runtime = ctx.attr.runtime[RuntimePackageInfo]
    subs = dict({
        "target": package(ctx.label),
        "kernel": ki.image.short_path,
        "rootfs": rootfs,
        "init": runtime.init,
        "runtime": runtime.root,
        "files": shell.array_literal([d.short_path for d in runtime.deps]),
    }, **extra)

    runfiles = ctx.runfiles(files = inputs.to_list())
    if runtime.check:
        subs["checker"] = runtime.check.binary.short_path
        runfiles = runfiles.merge(ctx.runfiles(files = [runtime.check.binary]))
        runfiles = runfiles.merge(runtime.check.runfiles)

    subs["code"] = code.format(**subs)
    executable = ctx.actions.declare_file(ctx.attr.name + "-start.sh")
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = executable,
        substitutions = dict([("{%s}" % (k), v) for k, v in subs.items()]),
        is_executable = True,
    )
    return [DefaultInfo(runfiles = runfiles, executable = executable)]
