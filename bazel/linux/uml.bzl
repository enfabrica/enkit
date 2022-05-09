load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "KernelModulesInfo", "KernelTreeInfo", "RootfsImageInfo", "RuntimePackageInfo")
load("@bazel_skylib//lib:shell.bzl", "shell")

def _kernel_uml_test(ctx):
    ki = ctx.attr.kernel_image[KernelImageInfo]

    inputs = depset(transitive = [
        ctx.attr.kernel_image.files,
        ctx.attr.runtime.files,
    ])

    rootfs = ""
    if ctx.attr.rootfs_image:
        rootfs = ctx.attr.rootfs_image[RootfsImageInfo].image.short_path
        inputs = depset(transitive = [inputs, ctx.attr.rootfs_image.files])

    runtime = ctx.attr.runtime[RuntimePackageInfo]
    subs = {
        "{kernel}": ki.image.short_path,
        "{rootfs}": rootfs,
        "{init}": runtime.init,
        "{runtime}": runtime.root,
        "{files}": shell.array_literal([d.short_path for d in runtime.deps]),
    }
    runfiles = ctx.runfiles(files = inputs.to_list())
    if runtime.check:
        subs["{checker}"] = runtime.check.binary.short_path
        runfiles = runfiles.merge(ctx.runfiles(files = [runtime.check.binary]))
        runfiles = runfiles.merge(runtime.check.runfiles)

    executable = ctx.actions.declare_file(ctx.attr.name + "-start.sh")
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = executable,
        substitutions = subs,
        is_executable = True,
    )
    return [DefaultInfo(runfiles = runfiles, executable = executable)]

kernel_uml_test = rule(
    doc = """Runs a test using the kunit framework.

kernel_test will retrieve the elements needed to setup a linux kernel test environment, and then execute the test.
The test will run locally inside a user-mode linux process.
""",
    implementation = _kernel_uml_test,
    attrs = {
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
            doc = "A target returning a RuntimePackageInfo, with tests to run in uml",
        ),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/run_uml.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
    },
    test = True,
)
