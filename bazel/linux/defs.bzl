def _kernel_tree_version(ctx):
    distro, version = ctx.attr.package.split("-", 1)

    ctx.download_and_extract(ctx.attr.url, output = ".", sha256 = ctx.attr.sha256, auth = ctx.attr.auth, stripPrefix = ctx.attr.strip_prefix)

    install_script = "install-" + version + ".sh"
    install_script_path = ctx.path(install_script)
    separator = "========================"
    if not install_script_path.exists:
        fail(
            """
{separator}
Could not find '{install_script}' inside the specified kernel package.
This usually means that you did not respect the naming convention of the package attribute of the kernel_tree_version rule:
* package should be something like 'distro-kernel_version-arch'
* the install script should be named 'install-kernel_version-arch.sh'
Read the kernel_tree_version doc for more info.
{separator}""".format(
                separator = separator,
                install_script = install_script,
            ),
        )

    result = ctx.execute([install_script_path])
    if result.return_code != 0:
        fail("""
{separator}
INSTALL SCRIPT FAILED

command: '{command}'
directory: '{directory}'
stdout: '{stdout}'
stderr: '{stderr}'
{separator}""".format(
            separator = separator,
            command = install_script_path,
            directory = ctx.path(""),
            stdout = result.stdout.strip(),
            stderr = result.stderr.strip(),
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

KernelModulesInfo = provider(
    doc = """Maintains the information necessary to represent compiled kernel modules.""",
    fields = {
        "name": "Name of the rule that defined this kernel module.",
        "package": "A string indicating which package this kernel module has been built against. For example, 'centos-kernel-5.3.0-1'.",
        "modules": "A list of files representing the compiled kernel modules (.ko).",
    },
)

def _import_kernel_bundle_dep(name, ki, d, inputs, extra_symbols):
    """Extracts information necessary to import a kernel bundle.

    A kernel bundle represents a single kernel module compiled for
    multiple kernels. Given a bundle d, a rule named name, trying
    to compile a module for kernel ki, updates inputs and extra_symbols
    with the correct list of dependencies.

    Args:
      name: string, name of the target.
      ki: KernelTreeInfo provider, describing the target kernel this
          module is being built for.
      d: a Target object, obtained from a label or label_list attribute,
         representing a dependency to link with this module.
      inputs: list of File objects, updated with additional inputs
         required by this dependency.
      extra_symbols: list of File objects, updated with additional
         Module.symvers files.

    Returns:
      Updates inputs and extra_symbols, returns None.
    """

    matched = 0
    for sub in d[KernelBundleInfo].modules:
        matched += int(_import_kernel_modules_dep(name, ki, sub, inputs, extra_symbols))

    if matched <= 0:
        fail("""
Module '{module}' is being built for kernel '{kernel}'.

PROBLEM: it has a dependency on module '{dependency}', but this target is NOT built for the same kernel!
Probably, you need to change module '{dependency}' so that it is also built for this same kernel.
""".format(
            module = name,
            kernel = ki.package,
            dependency = d.label.name,
        ))

def _import_kernel_modules_dep(name, ki, d, inputs, extra_symbols):
    """Extract information neessary to import a single kernel dep.

    Just like _import_kernel_bundle_dep, but works on a single module.

    Returns:
      True, if the module was compiled for the same kernel, and should
      be imported. False otherwise.
    """
    if d[KernelModulesInfo].package != ki.package:
        return False

    for f in d.files.to_list():
        if f.extension == "symvers":
            extra_symbols.append(f)

    inputs.extend(d.files.to_list())
    return True

def _kernel_modules(ctx):
    modules = ctx.attr.modules
    inputs = ctx.files.srcs + ctx.files.kernel
    srcdir = ctx.file.makefile.dirname
    extra_symbols = []

    ki = ctx.attr.kernel[KernelTreeInfo]

    for d in ctx.attr.deps:
        if KernelBundleInfo in d:
            _import_kernel_bundle_dep(ctx.attr.name, ki, d, inputs, extra_symbols)

        if KernelModulesInfo in d:
            _import_kernel_modules_dep(ctx.attr.name, ki, d, inputs, extra_symbols)

        if CcInfo in d:
            inputs += d[CcInfo].compilation_context.headers.to_list()
        inputs += d.files.to_list()

    outputs = []
    message = ""
    copy_command = ""
    for m in modules:
        message += "kernel building: compiling module %s for %s" % (m, ki.package)

        rename = m
        if ctx.attr.rename:
            rename = ctx.attr.rename + m
        output = ctx.actions.declare_file(rename)
        outputs += [output]
        copy_command += "cp {src_dir}/{module} {output_long} && ".format(
            src_dir = srcdir,
            module = m,
            output_long = output.path,
        )

        output = ctx.actions.declare_file(ctx.attr.rename + m + ".symvers")
        outputs += [output]
        copy_command += "cp {src_dir}/Module.symvers {output_long} && ".format(
            src_dir = srcdir,
            output_long = output.path,
        )
    copy_command += "true"

    kernel_build_dir = "{kr}/{kb}".format(kr = ki.root, kb = ki.build)

    extra = ""
    if ctx.attr.extra:
        extra = " ".join(ctx.attr.extra)

    extra_symbols = " ".join(["$PWD/" + e.path for e in extra_symbols])

    if extra_symbols:
        extra += " KBUILD_EXTRA_SYMBOLS=\"%s\"" % (extra_symbols)

    if ctx.attr.silent:
        silent = "-s"
    else:
        silent = ""

    make_args = ctx.attr.make_format_str.format(
        src_dir = srcdir,
        kernel_build_dir = kernel_build_dir,
        modules = " ".join(modules),
    )

    compilation_mode = ctx.var["COMPILATION_MODE"]
    if compilation_mode == "fastbuild":
        cflags = "-g"
    elif compilation_mode == "opt":
        cflags = ""
    elif compilation_mode == "dbg":
        cflags = "-g -O1 -fno-inline"
    else:
        fail("compilation mode '{compilation_mode}' not supported".format(
            compilation_mode=compilation_mode,
        ))

    extra += " EXTRA_CFLAGS='%s'" % cflags

    ctx.actions.run_shell(
        mnemonic = "KernelBuild",
        progress_message = message,
        command = "make {silent} {make_args} {extra_args} && {copy_command}".format(
            silent = silent,
            make_args = make_args,
            extra_args = extra,
            copy_command = copy_command,
        ),
        outputs = outputs,
        inputs = inputs,
        use_default_shell_env = True,
    )

    return [DefaultInfo(files = depset(outputs)), KernelModulesInfo(
        name = ctx.attr.name,
        package = ki.package,
        modules = outputs,
    )]

kernel_modules_rule = rule(
    doc = """Builds kernel modules.

The kernel_modules_rule will build the specified files as kernel
modules. As kernel modules must be built against a specific kernel,
the 'kernel' attribute must point to a rule created with 'kernel_tree'
or 'kernel_tree_version' (really, anything exporting a KernelTreeInfo
provider).

The attributes are pretty self explanatory. For convenience, though,
we recommend using the 'kernel_module' macro for building a single out
of tree kernel module, as that macro will provide convenient defaults
for you to save some error prone typing, and enjoy more time doing
whatever you do when not debugging flaky builds.
""",
    implementation = _kernel_modules,
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
        "modules": attr.string_list(
            mandatory = True,
            doc = "The list of kernel modules generated by the Makefile. If you are building a module 'e1000.ko', this would be the list ['e1000.ko'].",
        ),
        "make_format_str": attr.string(
            mandatory = True,
            doc = """Format string for generating 'make' command line arguments.

Available format values are:
{src_dir}          - source directory of the Makefile
{kernel_build_dir} - kernel build directory
{module}           - module name

""",
        ),
        "rename": attr.string(
            doc = "Prefix to apply to the module names at the end of the build. If not specified, the output files are not renamed.",
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

KernelBundleInfo = provider(
    doc = "Represents a set of the same module compiled for different kernels.",
    fields = {
        "modules": "List of module targets part of this bundle.",
    },
)

def _kernel_modules_bundle(ctx):
    return [DefaultInfo(files = depset(ctx.files.modules)), KernelBundleInfo(modules = ctx.attr.modules)]

kernel_modules_bundle = rule(
    doc = """Creates a bundle of kernel modules.

A bundle of kernel modules is a set of kernel modules which are ALL CONSIDERED
TO BE THE SAME KERNEL MODULE, but built for different kernel versions
or architecture.

You can then use a kernel_modules_bundle target to either build ALL the
kernel modules in the bundle, or as a dependency for building yet another
kernel module.

When used as a dependency, the logic in this file will cause the building
and linking step to pull in only the modules within the bundle that are
built for the same kernel (in the future: same architecture).

This is used for managing dependency chains of kernel modules more easily.

For example:
- You build a _core kernel module for 5 different kernels for your driver.
  These 5 kernel modules become a bundle.

- You create a new kernel module, only built for 1 specific kernel, that
  requires your _core module. By having this new module depend on a
  kernel_modules_bundle() _core module, the logic in this file will
  pick the correct symbols to link against for the specific kernel
  version, and error out if the dependency is not available in the
  version required.
""",
    implementation = _kernel_modules_bundle,
    attrs = {
        "modules": attr.label_list(
            mandatory = True,
            providers = [KernelModulesInfo],
            doc = "List of kernel modules part of this bundle",
        ),
    },
)

BUILD_LEFTOVERS = ["**/.*.cmd", "**/*.a", "**/*.o", "**/*.ko", "**/*.order", "**/*.symvers", "**/*.mod", "**/*.mod.c", "**/*.mod.o"]

def _kernel_module_targets(*args, **kwargs):
    """Common kernel module target setup."""

    modules = []
    for m in kwargs.get("modules", kwargs["name"]):
        if not m.endswith(".ko"):
            m = m + ".ko"
        modules += [m]
    kwargs["modules"] = modules

    kernels = kwargs.pop("kernels", [])
    if "kernel" in kwargs:
        kernels.append(kwargs.pop("kernel"))

    if len(kernels) == 1:
        kwargs["kernel"] = _normalize_kernel(kernels.pop())
        return kernel_modules_rule(*args, **kwargs)

    targets = []
    original = kwargs["name"]
    for kernel in kernels:
        kernel = _normalize_kernel(kernel)
        rename = kernel[1:] + "/"
        name = kernel[1:] + "-" + original
        targets.append(":" + name)

        kwargs["name"] = name
        kwargs["kernel"] = kernel
        kwargs["rename"] = rename
        kernel_modules_rule(*args, **kwargs)

    # This creates a target with the name chosen by the user that
    # builds all the modules for all the requested kernels at once.
    # Without this, the user can only build :all, or the specific
    # module for a specific kernel.
    return kernel_modules_bundle(name = original, modules = targets, visibility = kwargs.get("visibility"))

def kernel_module(*args, **kwargs):
    """Convenience wrapper around kernel_modules_rule.

    Use this wrpaper for building a single out of tree kernel module.

    The parameters passed to kernel_module are just passed to
    kernel_module_rule, except for what is listed below.

    Args:
      srcs: list of labels, specifying the source files that constitute the kernel module.
            If not specified, kernel_module will provide a reasonable default including all
            files that are typically part of a kernel module (i.e., the specified makefile
            and all .c and .h files belonging to the package where the kernel_module rule
            has been instantiated, see https://docs.bazel.build/versions/master/be/functions.html#glob).
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

    if "makefile" not in kwargs:
        kwargs["makefile"] = "Makefile"

    if "srcs" not in kwargs:
        include = ["**/*.c", "**/*.h", kwargs["makefile"]]
        kwargs["srcs"] = native.glob(include = include, exclude = BUILD_LEFTOVERS, allow_empty = False)

    kwargs["modules"] = [kwargs.pop("module", kwargs["name"])]

    if "make_format_str" not in kwargs:
        kwargs["make_format_str"] = "-C $PWD/{kernel_build_dir} M=$PWD/{src_dir} {modules}"

    return _kernel_module_targets(*args, **kwargs)

def nv_driver(*args, **kwargs):
    """Convenience wrapper around kernel_modules_rule.

    Use this wrpaper for building the NVidia driver modules.

    The parameters passed to nv_driver are just passed to nv_driver_rule, except for
    what is listed below.

    Args:
      srcs: list of labels, specifying the source files that constitute the kernel module.
            If not specified, nv_driver will provide a reasonable default including all
            files that are typically part of a kernel module (i.e., the specified makefile
            and all .c and .h files belonging to the package where the kernel_module rule
            has been instantiated, see https://docs.bazel.build/versions/master/be/functions.html#glob).
      modules: list of strings, naming the output modules. Mandatory. Also, it normalizes the
            names ensuring they have a '.ko' suffix.
      makefile: string, name of the makefile to build the driver. If not specified, nv_driver
            assumes it is just called Makefile.
      kernel: a label, indicating the kernel_tree to build the module against. nv_driver ensures
            the label starts with an '@', as per bazel convention.
      kernels: list of kernel (same as above). kernel_module will instantiate multiple
            kernel_module_rule, one per kernel, and ensure they all build in parallel.
    """

    if "makefile" not in kwargs:
        kwargs["makefile"] = "Makefile"

    if "srcs" not in kwargs:
        include = ["**/*.c", "**/*.h", "Kbuild", "**/*.Kbuild", "conftest.sh", "**/*.o_binary", kwargs["makefile"]]
        kwargs["srcs"] = native.glob(include = include, exclude = BUILD_LEFTOVERS, allow_empty = False)

    if "make_format_str" not in kwargs:
        kwargs["make_format_str"] = "-C $PWD/{src_dir} SYSSRC=$PWD/{kernel_build_dir} SYSOUT=$PWD/{kernel_build_dir} -j modules"

    return _kernel_module_targets(*args, **kwargs)

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
    mi = ctx.attr.module[KernelModulesInfo]

    # Kernel tests assume only one kernel module
    module = mi.modules[0]

    # Confirm that the kernel test module is compatible with the precompiled linux kernel executable image.
    if ki.package != mi.package:
        fail(
            "kernel_test expects a test kernel module built against the kernel tree package used to obtain the kernel executable image. " +
            "Instead it was given kernel_test.module.kernel.package='{}' and kernel_test.kernel_image.package='{}'.".format(mi.package, ki.package),
        )

    parser = ctx.attr._parser.files_to_run.executable
    inputs = [ki.image, ri.image, module, parser]
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
            "{module}": module.short_path,
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
            providers = [DefaultInfo, KernelModulesInfo],
            doc = "The label of the KUnit linux kernel module to be used for testing. It must define a kunit_test_suite so that when loaded, KUnit will start executing its tests.",
        ),
        "_template": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:run_um_kunit_tests.template"),
            doc = "The template to generate the bash script used to run the tests.",
        ),
        "_parser": attr.label(
            default = Label("//bazel/linux/kunit:kunit_zip"),
            doc = "KUnit TAP output parser.",
        ),
    },
    test = True,
)
