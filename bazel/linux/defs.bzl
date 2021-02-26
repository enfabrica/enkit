def _kernel_tree_version(ctx):
    distro, version = ctx.attr.package.split("-", 1)

    ctx.download_and_extract(ctx.attr.url, output = ".", sha256 = ctx.attr.sha256, auth = ctx.attr.auth, stripPrefix = ctx.attr.strip_prefix)

    command = ctx.path("install-" + version + ".sh")
    result = ctx.execute([command])

    sep = "\n========================"
    if result.return_code != 0:
        fail("%s\nINSTALL FAILED\ncommand: %s\ndirectory: %s\nstdout:\n%s\nstderr:\n%s%s" % (
            sep,
            command,
            ctx.path(""),
            result.stdout,
            result.stderr,
            sep,
        ))

    ctx.template(
        "BUILD.bazel",
        ctx.attr._template,
        substitutions = {
            "{name}": ctx.name,
            "{package}": ctx.attr.package,
            "{build}": "%s/build" % (result.stdout.strip()),
            "{utils}": str(ctx.attr._utils),
        },
        executable = False,
    )

kernel_tree_version = repository_rule(
    doc = """Imports a specific kernel version to build out of tree modules.

A kernel_tree_version rule will download a specific kernel version and make it available
to the rest of the repository to build kernel modules.

kernel_version rules are repository_rule, meaning that they are meant to be used from
within a WORKSPACE file to download dependencies before the build starts.

As an example, you can use:

    kernel_tree_version(
        name = "default-kernel",
        package = "debian-5.9.0-rc6-amd64",
        url = "astore.corp.enfabrica.net/d/kernel/debian/5.9.0-build893849392.tar.gz",
    )

To download the specified .tar.gz from "https://astore.corp.enfabrica.net/d/kernel",
and use it as the "default-kernel" from the repository.

Note that this rule expects a "pre-processed" kernel package: the .tar.gz above
will be a slice of the kernel tree, containing a .config file and a bunch of
other pre-compiled tools, ready to build a kernel specifically for debian
(or the distribution picked).

To create a .tar.gz suitable for this rule, you can use the kbuild tool, available at:

    https://github.com/enfabrica/enkit/kbuild
""",
    implementation = _kernel_tree_version,
    local = False,
    attrs = {
        "package": attr.string(
            doc = "The name of the downloaded kernel. Format is 'distribution-kernel-version-arch', like debian-5.9.0-rc6-rt-amd64.",
            mandatory = True,
        ),
        "url": attr.string(
            doc = "The url to download the package from.",
            mandatory = True,
        ),
        "sha256": attr.string(
            doc = "The sha256 of the downloaded package file.",
        ),
        "auth": attr.string_dict(
            doc = "An auth dict as documented for the download_and_extract context rule as is.",
        ),
        "strip_prefix": attr.string(
            doc = "A path prefix to remove after unpackaging the file, passed to the download_and_extract context rule as is.",
        ),
        "_template": attr.label(
            default = Label("//bazel/linux:kernel_tree.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

KernelTreeInfo = provider(
    doc = """Maintains the information necessary to build a module out of a kernel tree.

In a rule(), you will generally want to create a 'make' command using 'make ... -C $root/$build ...'.
Note that the kernel tree may depend on tools or ABIs not installed/available on your system,
a kernel_tree on its own is not expected to be hermetic.
""",
    fields = {
        "name": "Name of the rule that defined this kernel tree. For example, 'carlo-s-favourite-kernel'.",
        "package": "A string indicating which package this kernel is coming from. For example, 'centos-kernel-5.3.0-1'.",
        "root": "Bazel directory containing the root of the kernel tree. This is generally the location of the top level BUILD.bazel file. For example, external/@centos-kernel-5.3.0-1.",
        "build": "Relative path of subdirectory to enter to build a kernel module. It is generally the 'build' parameter passed to the kernel_tree rule. For example, lib/modules/centos-kernel-5.3.0-1/build.",
    },
)

KernelModuleInfo = provider(
    doc = """Maintains the information necessary to represent a compiled kernel module.""",
    fields = {
        "name": "Name of the rule that defined this kernel module.",
        "package": "A string indicating which package this kernel module has been built against. For example, 'centos-kernel-5.3.0-1'.",
        "module": "File representing the compiled kernel module (.ko).",
    },
)

def _kernel_module(ctx):
    module = ctx.attr.module
    inputs = ctx.files.srcs + ctx.files.kernel
    srcdir = ctx.file.makefile.dirname

    for d in ctx.attr.deps:
        inputs += d.files.to_list()
        if CcInfo in d:
            inputs += d[CcInfo].compilation_context.headers.to_list()

    rename = ctx.attr.rename
    if not rename:
        rename = module
    output = ctx.actions.declare_file(rename)

    extra = ""
    if ctx.attr.extra:
        extra = " ".join(ctx.attr.extra)

    ki = ctx.attr.kernel[KernelTreeInfo]
    ctx.actions.run_shell(
        mnemonic = "KernelBuild",
        progress_message = "kernel building: compiling kernel module %s for %s" % (module, ki.package),
        command = """make -s M=$PWD/{src_dir} -C $PWD/{kernel_root}/{build_dir} {module} {extra_args} &&
                     cp $PWD/{src_dir}/{module} {output_long} &&
                     echo ==== NO FATAL ERRORS - MODULE CREATED - bazel-bin/{output_short}""".format(
            src_dir = srcdir,
            kernel_root = ki.root,
            build_dir = ki.build,
            extra_args = extra,
            module = module,
            output_long = output.path,
            output_short = output.short_path,
        ),
        outputs = [output],
        inputs = inputs,
        use_default_shell_env = True,
    )

    return [DefaultInfo(files = depset([output])), KernelModuleInfo(
        name = ctx.attr.name,
        package = ki.package,
        module = output,
    )]

kernel_module_rule = rule(
    doc = """Builds a kernel module.

The kernel_module_rule will build the specified files as a kernel module. As kernel modules must be built
against a specific kernel, the 'kernel' attribute must point to a rule created with 'kernel_tree' or 'kernel_tree_version'
(really, anything exporting a KernelTreeInfo provider).

The attributes are pretty self explanatory. For convenience, though, we recommend using the
'kernel_module' macro rather than 'kernel_module_tree', as that macro will provide convenient
defaults for you to save some error prone typing, and enjoy more time doing whatever you do
when not debugging flaky builds.
""",
    implementation = _kernel_module,
    attrs = {
        "kernel": attr.label(
            mandatory = True,
            providers = [DefaultInfo, KernelTreeInfo],
            doc = "The kernel to build this module against. A string like @carlo-s-favourite-kernel, referencing a kernel_tree_version(name = 'carlo-s-favourite-kernel', ...",
        ),
        "makefile": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "A label pointing to the Makefile to use. Unless you are doing anything funky, normally you would have the string 'Makefile' here.",
        ),
        "module": attr.string(
            mandatory = True,
            doc = "The name of the file generated by the Makefile. If you are building a module 'e1000.ko', this would be the string 'e1000.ko'.",
        ),
        "rename": attr.string(
            doc = "How you want the module to be named at the end of the build. If not specified, the output file is not renamed. Building the file multiple times will require different names.",
        ),
        "silent": attr.bool(
            default = True,
            doc = "If set to False, the standard kernel 'make' output will be let free to clobber your console.",
        ),
        "deps": attr.label_list(
            doc = "List of additional dependencies necessary to build this module.",
        ),
        "extra": attr.string_list(
            doc = "Anything more you'd like to pass to 'make'. All arguments specified here are just appended at the end of the build.",
        ),
        "srcs": attr.label_list(
            mandatory = True,
            allow_empty = False,
            allow_files = True,
            doc = "The list of files that constitute this module. Generally a glob for all .c and .h files. If you use **/* with glob, we recommend excluding the patterns defined by BUILD_LEFTOVERS.",
        ),
    },
)

def _normalize_kernel(kernel):
    """Ensures a kernel string points to a repository rule, with bazel @ syntax."""
    if not kernel.startswith("@"):
        kernel = "@" + kernel

    return kernel

BUILD_LEFTOVERS = [".*.cmd", "*.a", "*.o", "*.ko", "*.order", "*.symvers", "*.mod", "*.mod.[co]"]

def kernel_module(*args, **kwargs):
    """Convenience wrapper around kernel_module_rule, makes it easier to use.

    The parameters passed to kernel_module are just passed to kernel_module_rule, except for
    what is listed below.

    Args:
      srcs: list of labels, specifying the source files that constitute the kernel module.
            If not specified, kernel_module will provide a reasonable default including all
            files that are typically part of a kernel module.
      module: string, name of the output module. If not specified, kernel_module will assume
            the output module name will be the same as the rule name. Also, it normalizes the
            name ensuring it has a '.ko' suffix.
      makefile: string, name of the makefile to build the module. If not specified, kernel_module
            assumes it is just called Makefile.
      kernel: a label, indicating the kernel_tree to build the module against. kernel_module ensures
            the label starts with an '@', as per bazel convention.
      kernels: list of kernel (same as above). kernel_module will instantiate multiple
            kernel_module_rule, one per kernel, and ensure they all build in parallel.
    """
    if "srcs" not in kwargs:
        kwargs["srcs"] = native.glob(include = ["**/*"], exclude = BUILD_LEFTOVERS, allow_empty = False)

    module = kwargs.get("module", kwargs["name"])
    if not module.endswith(".ko"):
        module = module + ".ko"
    kwargs["module"] = module

    if "makefile" not in kwargs:
        kwargs["makefile"] = "Makefile"

    if "kernels" not in kwargs:
        kwargs["kernel"] = _normalize_kernel(kwargs["kernel"])
        return kernel_module_rule(*args, **kwargs)

    kernels = kwargs["kernels"]
    kwargs.pop("kernels")

    targets = []
    original = kwargs["name"]
    for kernel in kernels:
        kernel = _normalize_kernel(kernel)
        rename = kernel[1:] + "/" + module
        name = kernel[1:] + "-" + original
        targets.append(":" + name)

        kwargs["name"] = name
        kwargs["kernel"] = kernel
        kwargs["rename"] = rename
        kernel_module_rule(*args, **kwargs)

    # This creates a target with the name chosen by the user that builds all the kernel modules at once.
    # Without this, the user can only build :all, or the specific module for a specific kernel.
    return native.filegroup(name = original, srcs = targets)

def _kernel_tree(ctx):
    return [DefaultInfo(files = depset(ctx.files.files)), KernelTreeInfo(
        name = ctx.attr.name,
        package = ctx.attr.package,
        root = ctx.label.workspace_root,
        build = ctx.attr.build,
    )]

kernel_tree = rule(
    doc = """Defines a new kernel tree.

This rule exports a set of files that represent a partial linux kernel tree
with just enough files and tools to build an out-of-tree kernel modules.

kernel_tree rules are typically automatically created when you declare a
kernel_tree_version() in your WORKSPACE file. You should almost never have to create
kernel_tree rules manually.

The only exception is if you check in directly in your repository a patched
version of the linux kernel to build your own modules with.

All KernelTree rules export a KernelTreeInfo provider.

Example:

    kernel_tree(
        # An arbitrary name for the rule.
        name = "carlo-s-favourite-kernel",
        # The package this kernel is coming from.
        package = "centos-kernel-5.3.0-1",
        # To build modules for this kernel, this is the subdirectory to enter.
        build = "lib/modules/5.3.0-1/build",
        # This kernel tree is made by all the files here, nothing excluded.
        files = glob(["**/*"]),
    )
""",
    implementation = _kernel_tree,
    attrs = {
        "files": attr.label_list(
            allow_empty = False,
            doc = "Files that constitute this kernel tree, and necessary to build modules.",
        ),
        "package": attr.string(
            mandatory = True,
            doc = "A string indicating which package this kernel is coming from.",
        ),
        "build": attr.string(
            mandatory = True,
            doc = "Relative path of subdirectory to enter to build modules. Used to compute the path for 'make -C ...'.",
        ),
    },
)

RootfsImageInfo = provider(
    doc = """Maintains the information necessary to represent a rootfs image.
""",
    fields = {
        "name": "Name of the rule that defined this rootfs image. For example, 'stefano-s-favourite-rootfs'.",
        "image": "File containing the rootfs image.",
    },
)

def _rootfs_image(ctx):
    return [DefaultInfo(files = depset([ctx.file.image])), RootfsImageInfo(
        name = ctx.attr.name,
        image = ctx.file.image,
    )]

rootfs_image = rule(
    doc = """Defines a new rootfs image.

This rule exports a file that represents a linux rootfs image with just enough
to be able to boot a linux executable image.

rootfs_image rules are typically automatically created when you declare a
rootfs_version() in your WORKSPACE file. You should almost never have to create
rootfs_image rules manually.

The only exception is if you want to troubleshoot a new rootfs image you have
available locally.

All RootfsImage rules export a RootfsImageInfo provider.

Example:

    rootfs_image(
        # An arbitrary name for the rule.
        name = "stefano-s-favourite-rootfs",
        # This rootfs image file.
        image = "buildroot-custom-amd64.img",
    )
""",
    implementation = _rootfs_image,
    attrs = {
        "image": attr.label(
            mandatory = True,
            allow_single_file = True,
            doc = "File containing the rootfs image.",
        ),
    },
)

def _rootfs_version(ctx):
    ctx.download(ctx.attr.url, output = ctx.attr.package, sha256 = ctx.attr.sha256, auth = ctx.attr.auth)
    ctx.template(
        "BUILD.bazel",
        ctx.attr._template,
        substitutions = {
            "{name}": ctx.name,
            "{image}": ctx.attr.package,
            "{utils}": str(ctx.attr._utils),
        },
        executable = False,
    )

rootfs_version = repository_rule(
    doc = """Imports a specific rootfs version to be used for kernel tests.

A rootfs_version rule will download a specific rootfs version and make it available
to the rest of the repository to generate kunit tests environments.

rootfs_version rules are repository_rule, meaning that they are meant to be used from
within a WORKSPACE file to download dependencies before the build starts.

As an example, you can use:

    rootfs_version(
        name = "test-latest-rootfs",
        package = "buildroot-custom-amd64",
        url = "astore.corp.enfabrica.net/d/kernel/test/buildroot-custom-amd64.img",
    )

To download the specified image from "https://astore.corp.enfabrica.net/d/kernel",
and use it as the "test-latest-rootfs" from the repository.
""",
    implementation = _rootfs_version,
    local = False,
    attrs = {
        "package": attr.string(
            doc = "The name of the downloaded image. Usually the format is 'distribution-rootfs_version-arch', like buildroot-custom-amd64.",
            mandatory = True,
        ),
        "url": attr.string(
            doc = "The url to download the rootfs image from.",
            mandatory = True,
        ),
        "sha256": attr.string(
            doc = "The sha256 of the downloaded package file.",
        ),
        "auth": attr.string_dict(
            doc = "An auth dict as documented for the download_and_extract context rule as is.",
        ),
        "_template": attr.label(
            default = Label("//bazel/linux:rootfs.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

KernelImageInfo = provider(
    doc = """Maintains the information necessary to represent a kernel executable image.""",
    fields = {
        "name": "Name of the rule that defined this kernel executable image. For example, 'stefano-s-favourite-kernel-image'.",
        "package": "A string indicating which package this kernel executable image is coming from. For example, 'custom-5.9.0-um'.",
        "image": "Path of the kernel executable image.",
    },
)

def _kernel_image(ctx):
    return [DefaultInfo(files = depset([ctx.file.image])), KernelImageInfo(
        name = ctx.attr.name,
        package = ctx.attr.package,
        image = ctx.file.image,
    )]

kernel_image = rule(
    doc = """Defines a new kernel executable image.

This rule exports a file that represents a kernel executable image with just
enough to be able to run kernel tests.

kernel_image rules are typically automatically created when you declare a
kernel_image_version() in your WORKSPACE file. You should almost never have to
create kernel_image rules manually.

The only exception is if you want to troubleshoot a new kernel image you have
available locally.

Example:

    kernel_image(
        # An arbitrary name for the rule.
        name = "stefano-s-favourite-kernel-image",
        # This kernel image file.
        image = "custom-5.9.0-um",
    )
""",
    implementation = _kernel_image,
    attrs = {
        "package": attr.string(
            mandatory = True,
            doc = "A string indicating which package this kernel executable image is coming from.",
        ),
        "image": attr.label(
            mandatory = True,
            executable = True,
            cfg = "target",
            allow_single_file = True,
            doc = "File containing the kernel executable image.",
        ),
    },
)

def _kernel_image_version(ctx):
    ctx.download(
        ctx.attr.url,
        output = ctx.attr.package,
        sha256 = ctx.attr.sha256,
        auth = ctx.attr.auth,
        executable = True,
    )
    ctx.template(
        "BUILD.bazel",
        ctx.attr._template,
        substitutions = {
            "{name}": ctx.name,
            "{package}": ctx.attr.package,
            "{image}": ctx.attr.package,
            "{utils}": str(ctx.attr._utils),
        },
        executable = False,
    )

kernel_image_version = repository_rule(
    doc = """Imports a specific kernel executable image version to be used for kernel tests.

A kernel_image_version rule will download a specific kernel image version and make it available
to the rest of the repository to generate kernel modules tests environments.

kernel_image_version rules are repository_rule, meaning that they are meant to be used from
within a WORKSPACE file to download dependencies before the build starts.

As an example, you can use:

    kernel_image_version(
        name = "test-latest-kernel-image",
        package = "custom-5.9.0-um",
        url = "astore.corp.enfabrica.net/d/kernel/test/custom-5.9.0-um",
    )

To download the specified image from "https://astore.corp.enfabrica.net/d/kernel",
and use it as the "test-latest-kernel-image" from the repository.

To create an image suitable for this rule, you can compile a linux source tree using your preferred configs.
""",
    implementation = _kernel_image_version,
    local = False,
    attrs = {
        "package": attr.string(
            doc = "The name of the downloaded image. Usually the format is 'distribution-kernel_version-arch', like custom-5.9.0-um.",
            mandatory = True,
        ),
        "url": attr.string(
            doc = "The url to download the kernel executable image from.",
            mandatory = True,
        ),
        "sha256": attr.string(
            doc = "The sha256 of the downloaded package file.",
        ),
        "auth": attr.string_dict(
            doc = "An auth dict as documented for the download_and_extract context rule as is.",
        ),
        "_template": attr.label(
            default = Label("//bazel/linux:kernel_image.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

def _kernel_test(ctx):
    ki = ctx.attr.kernel_image[KernelImageInfo]
    ri = ctx.attr.rootfs_image[RootfsImageInfo]
    mi = ctx.attr.module[KernelModuleInfo]

    # Confirm that the kernel test module is compatible with the precompiled linux kernel executable image.
    if ki.package != mi.package:
        fail(
            "kernel_test expects a test kernel module built against the kernel tree package used to obtain the kernel executable image. " +
            "Instead it was given kernel_test.module.kernel.package='{}' and kernel_test.kernel_image.package='{}'.".format(mi.package, ki.package),
        )

    parser = ctx.attr._parser.files_to_run.executable
    inputs = [ki.image, ri.image, mi.module, parser]
    inputs = depset(inputs, transitive = [
        ctx.attr.kernel_image.files,
        ctx.attr.rootfs_image.files,
        ctx.attr.module.files,
        ctx.attr._parser.files,
    ])
    executable = ctx.actions.declare_file("script.sh")
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = executable,
        substitutions = {
            "{kernel}": ki.image.short_path,
            "{rootfs}": ri.image.short_path,
            "{module}": mi.module.short_path,
            "{parser}": parser.short_path,
        },
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = inputs.to_list())
    runfiles = runfiles.merge(ctx.attr._parser.default_runfiles)
    return [DefaultInfo(runfiles = runfiles, executable = executable)]

kernel_test = rule(
    doc = """Test a linux kernel module using the KUnit framework.

kernel_test will retrieve the elements needed to setup a linux kernel test environment, and then execute the test.
The test will run locally inside a user-mode linux process.
""",
    implementation = _kernel_test,
    attrs = {
        "kernel_image": attr.label(
            mandatory = True,
            providers = [DefaultInfo, KernelImageInfo],
            doc = "The kernel image that will be used to execute this test. A string like @stefano-s-favourite-kernel-image, referencing a kernel_image(name = 'stefano-s-favourite-kernel-image', ...",
        ),
        "rootfs_image": attr.label(
            mandatory = True,
            providers = [DefaultInfo, RootfsImageInfo],
            doc = "The rootfs image that will be used to execute this test. A string like @stefano-s-favourite-rootfs-image, referencing a rootfs_image(name = 'stefano-s-favourite-rootfs-image', ...",
        ),
        "module": attr.label(
            mandatory = True,
            providers = [DefaultInfo, KernelModuleInfo],
            doc = "The label of the KUnit linux kernel module to be used for testing. It must define a kunit_test_suite so that when loaded, KUnit will start executing its tests.",
        ),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:run_um_kunit_tests.template"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_parser": attr.label(
            default = Label("//bazel/linux/kunit:kunit"),
            doc = "KUnit TAP output parser.",
        ),
    },
    test = True,
)
