load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "KernelModulesInfo", "KernelTreeInfo", "RootfsImageInfo", "RuntimePackageInfo")
load("@bazel_skylib//lib:shell.bzl", "shell")

def _kernel_uml_test(ctx):
    ki = ctx.attr.kernel_image[KernelImageInfo]

    inputs = depset(transitive = [
        ctx.attr.kernel_image.files,
        ctx.attr.runtime.files,
        ctx.attr._parser.files,
    ])

    rootfs = ""
    if ctx.attr.rootfs_image:
        rootfs = ctx.attr.rootfs_image[RootfsImageInfo].image.short_path
        inputs = depset(transitive = [inputs, ctx.attr.rootfs_image.files])

    runtime = ctx.attr.runtime[RuntimePackageInfo]
    executable = ctx.actions.declare_file(ctx.attr.name + "-start.sh")
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = executable,
        substitutions = {
            "{kernel}": ki.image.short_path,
            "{rootfs}": rootfs,
            "{parser}": ctx.executable._parser.short_path,
            "{init}": runtime.init,
            "{runtime}": runtime.root,
            "{files}": shell.array_literal([d.short_path for d in runtime.deps]),
        },
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = inputs.to_list())
    runfiles = runfiles.merge(ctx.attr._parser.default_runfiles)
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
            default = Label("//bazel/linux:templates/run_um_kunit_tests.template.sh"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_parser": attr.label(
            default = Label("//bazel/linux/kunit:kunit_zip"),
            doc = "KUnit TAP output parser.",
            executable = True,
            cfg = "host",
        ),
    },
    test = True,
)
