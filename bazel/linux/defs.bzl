load("//bazel/linux:uml.bzl", "kernel_uml_test")
load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "KernelModulesInfo", "KernelTreeInfo", "RootfsImageInfo", "RuntimePackageInfo")
load("//bazel/utils:messaging.bzl", "location", "package")
load("//bazel/utils:files.bzl", "files_to_dir")
load("//bazel/utils:macro.bzl", "mconfig", "mcreate_rule")
load("@bazel_skylib//lib:shell.bzl", "shell")
load("//bazel/astore:defs.bzl", "astore_download_and_extract")

def _kernel_tree_version(ctx):
    distro, version = ctx.attr.package.split("-", 1)

    if ctx.attr.url and not (ctx.attr.path or ctx.attr.uid):
        ctx.download_and_extract(ctx.attr.url, output = ".", sha256 = ctx.attr.sha256, auth = ctx.attr.auth, stripPrefix = ctx.attr.strip_prefix)
    elif (ctx.attr.path and ctx.attr.uid) and not ctx.attr.url:
        astore_download_and_extract(ctx, digest = ctx.attr.sha256, stripPrefix = ctx.attr.strip_prefix)

    else:
        fail("WORKSPACE repository {}: Provide either a URL, OR an astore path and UID".format(ctx.attr.name))


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
            doc = "The URL to download the package from. This is mutually exclusive with the astore path/uid arguments.",
        ),
        "path": attr.string(
            doc = "The astore path to download the package from.",
        ),
        "uid": attr.string(
            doc = "The astore UID for this package.",
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
            default = Label("//bazel/linux:templates/kernel_tree.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

def is_module(dep):
    """Returns True if this dependency is considered a kernel module."""
    return KernelModulesInfo in dep or KernelBundleInfo in dep

def is_compatible(arch, pkg, dep):
    """Checks if the supplied KernelModulesInfo matches the parameters."""
    if dep.package == pkg and dep.arch == arch:
        return [dep]
    return []

def get_compatible(ctx, arch, pkg, dep):
    """Extracts the set of compatible dependencies from a single dependency.

    The same kernel module can be built against multiple kernel trees and
    architectures. When that happens, it is packed in a KernelBundleInfo.

    When a build has a dependency on a KernelBundleInfo or even a plain
    kernel module through KernelModulesInfo, the build requires finding the
    specific .ko .symvers files for linking - the ones matching the same kernel
    and arch being built with the current target.

    Given a dependency dep, either a KernelBundleInfo or a KernelModuleInfo,
    this macro returns the set of KernelModuleInfo to build against, or
    an error.

    Args:
      ctx: a bazel rule ctx, for error printing.
      arch: string, desired architecture.
      pkg: string, desired kernel package.
      dep: a KernelModuleInfo or KernelBundleInfo to check.

    Returns:
      List of KernelModuleInfo to link against.
    """
    mods = []
    builtfor = []
    if KernelModulesInfo in dep:
        di = dep[KernelModulesInfo]
        mods.extend(is_compatible(arch, pkg, di))
        builtfor.append((di.package, di.arch))

    if KernelBundleInfo in dep:
        for module in dep[KernelBundleInfo].modules:
            mods.extend(is_compatible(arch, pkg, module))
            builtfor.append((module.package, module.arch))

    # Confirm that the kernel test module is compatible with the precompiled linux kernel executable image.
    if not mods:
        fail("\n\nERROR: " + location(ctx) + "requires {module} to be built for\n  kernel:{kernel} arch:{arch}\nBut it is only configured for:\n  {built}".format(
            arch = arch,
            kernel = pkg,
            module = package(dep.label),
            built = "\n  ".join(["kernel:{pkg} arch:{arch}".format(pkg = pkg, arch = arch) for pkg, arch in builtfor]),
        ))

    return mods

def _kernel_modules(ctx):
    modules = ctx.attr.modules
    srcdir = ctx.file.makefile.dirname

    if not ctx.attr.archs:
        fail(location(ctx) + "rule must specify one or more architectures in 'arch'")

    ki = ctx.attr.kernel[KernelTreeInfo]
    bundled = []
    for arch in ctx.attr.archs:
        inputs = ctx.files.srcs + ctx.files.kernel
        extra_symbols = []

        kdeps = []
        for d in ctx.attr.kdeps:
            kdeps.extend(get_compatible(ctx, arch, ki.package, d))

        for d in ctx.attr.deps:
            inputs.extend(d.files.to_list())

            if not is_module(d):
                if CcInfo in d:
                    inputs += d[CcInfo].compilation_context.headers.to_list()
            else:
                mods = get_compatible(ctx, arch, ki.package, d)

                kdeps.extend(mods)
                for mod in mods:
                    extra_symbols.extend([f for f in mod.files if f.extension == "symvers"])

        outputs = []
        message = ""
        copy_command = ""
        for m in modules:
            message += "kernel: compiling %s for arch:%s kernel:%s" % (m, arch, ki.package)

            outfile = "{kernel}/{arch}/{name}".format(
                kernel = ki.name,
                arch = arch,
                name = m,
            )

            output = ctx.actions.declare_file(outfile)
            outputs += [output]
            copy_command += "cp {src_dir}/{module} {output_long} && ".format(
                src_dir = srcdir,
                module = m,
                output_long = output.path,
            )

            output = ctx.actions.declare_file(outfile + ".symvers")
            outputs += [output]
            copy_command += "cp {src_dir}/Module.symvers {output_long} && ".format(
                src_dir = srcdir,
                output_long = output.path,
            )
        copy_command += "true"

        kernel_build_dir = "{kr}/{kb}".format(kr = ki.root, kb = ki.build)

        extra = []
        if arch != "host":
            extra.append("ARCH=" + arch)
        if ctx.attr.extra:
            extra += ctx.attr.extra

        extra_symbols = " ".join(["$PWD/" + e.path for e in extra_symbols])

        if extra_symbols:
            extra.append("KBUILD_EXTRA_SYMBOLS=\"%s\"" % (extra_symbols))

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
                compilation_mode = compilation_mode,
            ))

        extra.append("EXTRA_CFLAGS='%s'" % cflags)

        ctx.actions.run_shell(
            mnemonic = "KernelBuild",
            progress_message = message,
            command = "make {silent} {make_args} {extra_args} && {copy_command}".format(
                silent = silent,
                make_args = make_args,
                extra_args = " ".join(extra),
                copy_command = copy_command,
            ),
            outputs = outputs,
            inputs = inputs,
            use_default_shell_env = True,
        )

        bundled.append(KernelModulesInfo(
            label = ctx.label,
            arch = arch,
            package = ki.package,
            files = outputs,
            kdeps = kdeps,
            setup = ctx.attr.setup,
        ))

    return [DefaultInfo(files = depset(outputs)), KernelBundleInfo(modules = bundled)]

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
        "archs": attr.string_list(
            default = ["host"],
            doc = "The set of architectures supported by this module. Only the architectures needed will be built",
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
        "kdeps": attr.label_list(
            doc = "Additional dependencies needed at *run time* to load this module. Modules listed in dep are automatically added.",
            providers = [[KernelModulesInfo], [KernelBundleInfo]],
        ),
        "setup": attr.string_list(
            doc = "Some kernel modules require extra commands in order to be loaded. This attribute allows to define those shell commands.",
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

def _kernel_modules_bundle(ctx):
    modules = []
    for module in ctx.attr.modules:
        if KernelModulesInfo in module:
            modules.append(module)
        elif KernelBundleInfo in module:
            modules.extend(module[KernelBundleInfo].modules)

    return [DefaultInfo(files = depset(ctx.files.modules)), KernelBundleInfo(modules = modules)]

kernel_modules_bundle = rule(
    doc = """Creates a bundle of kernel modules.

A bundle of kernel modules is a set of kernel modules which are ALL CONSIDERED
TO BE THE SAME KERNEL MODULE, but built for different kernel versions or
architecture.

You can then use a kernel_modules_bundle target to either build ALL the
kernel modules in the bundle, or as a dependency for building yet another
kernel module.

When used as a dependency, the logic in this file will cause the building
and linking step to pull in only the modules within the bundle that are
necessary for the specific build, based on kernel and architecture.

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
            providers = [[KernelModulesInfo], [KernelBundleInfo]],
            doc = """\
List of kernel modules or bundles to be included in this bundle.

A bundle, however, is not allowed to contain another bundle. So
if one bundle is specified as a depencdency, it is transparently
expanded in its list of modules.""",
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
        name = kernel[1:] + "-" + original
        targets.append(":" + name)

        kwargs["name"] = name
        kwargs["kernel"] = kernel
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
            default = Label("//bazel/linux:templates/rootfs.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

def _kernel_image(ctx):
    return [DefaultInfo(files = depset([ctx.file.image])), KernelImageInfo(
        name = ctx.attr.name,
        package = ctx.attr.package,
        image = ctx.file.image,
        arch = ctx.attr.arch,
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
        # Architecture of this image file.
        arch = "um",
    )
""",
    implementation = _kernel_image,
    attrs = {
        "package": attr.string(
            mandatory = True,
            doc = "A string indicating which package this kernel executable image is coming from.",
        ),
        "arch": attr.string(
            doc = "Architecture this image was built for. Will only accept moudules for this arch.",
            default = "host",
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
            "{arch}": ctx.attr.arch,
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
        "arch": attr.string(
            doc = "The architecture this image was built for. 'host' means the architecture of the current machine.",
            default = "host",
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
            default = Label("//bazel/linux:templates/kernel_image.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
        ),
    },
)

def _expand_deps(ctx, mods, depth):
    """Recursively expands the dependencies of a module.

    Args:
      ctx: a Bazel ctx object, used to print useful error messages.
      mods: list of KernelModulesInfo, modules to compute the dependencies of.
      depth: int, how deep to go in recursively expanding the list of modules.

    Returns:
      List of KernelModulesInfo de-duplicated, in an order where insmod
      as a chance to successfully resolve the dependencies.
    """
    error = """ERROR:

While recurisvely expanding the chain of 'kdeps' for the module,
at {depth} iterations there were still dependencies to expand.
Loop? Or adjust the 'depth = ' attribute setting the max.

The modules computed to load so far are:
  {expanded}

The modules still to expand are:
  {current}
"""
    alldeps = list(mods)
    current = list(mods)

    for it in range(depth):
        pending = []
        for mi in current:
            for kdep in mi.kdeps:
                pending.append(kdep)

        alldeps.extend(pending)
        current = pending

        # Error out if we still have not expanded the full set of
        # dependencies at the last iteration.
        if current and it >= depth - 1:
            fail(location(ctx) + error.format(
                depth = depth,
                expanded = [package(d.label) for d in alldeps],
                current = [package(d.label) for d in current],
            ))

    # alldeps here lists all the recurisve dependencies of the supplied
    # mods starting from the module themselves.
    #
    # But... kernel module loading requires having the dependencies loaded
    # first. insmod may also fail if a module is loaded twice.
    #
    # So: reverse the list, remove duplicates.
    dups = {}
    result = []
    for mod in reversed(alldeps):
        key = package(mod.label)
        if key in dups:
            continue
        result.append(mod)

    return result

def _kernel_test_dir(ctx):
    ki = ctx.attr.image[KernelImageInfo]
    mods = get_compatible(ctx, ki.arch, ki.package, ctx.attr.module)
    alldeps = _expand_deps(ctx, mods, ctx.attr.depth)

    commands = []
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

    init = ctx.actions.declare_file(ctx.attr.name + "-init.sh")
    ctx.actions.expand_template(
        template = ctx.file._template_init,
        output = init,
        substitutions = {
            "{relpath}": init.short_path,
            "{commands}": "\n".join(commands),
        },
        is_executable = True,
    )

    check = ctx.actions.declare_file(ctx.attr.name + "-check.sh")
    ctx.actions.expand_template(
        template = ctx.file._template_check,
        output = check,
        substitutions = {
            "{parser}": ctx.executable._parser.short_path,
        },
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = ctx.attr._parser.files.to_list())
    runfiles = runfiles.merge(ctx.attr._parser.default_runfiles)

    d = files_to_dir(
        ctx,
        ctx.attr.name + "-root",
        inputs + [init],
        post = "cd {dest}; cp -L %s ./init.sh" % (shell.quote(init.short_path)),
    )
    return [
        DefaultInfo(files = depset([init, d, check]), runfiles = runfiles),
        RuntimePackageInfo(init = init.short_path, root = d.short_path, deps = [d], check = struct(
            binary = check,
            runfiles = runfiles,
        )),
    ]

kernel_test_dir = rule(
    doc = """\
Generates a directory containing the kernel module, all its dependencies,
and an init script to run them as a kunit test.""",
    implementation = _kernel_test_dir,
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
        "_template_init": attr.label(
            allow_single_file = True,
            default = Label("//bazel/linux:templates/init.template.sh"),
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

def kernel_test(name, kernel_image, module, rootfs_image = None, test_dir_cfg = {}, runner_cfg = {}, runner = kernel_uml_test, **kwargs):
    runtime = mcreate_rule(
        name,
        kernel_test_dir,
        "runtime",
        test_dir_cfg,
        kwargs,
        mconfig(module = module, image = kernel_image),
    )

    cfg = mconfig(runtime = runtime, kernel_image = kernel_image)
    if rootfs_image:
        cfg = mconfig(cfg, rootfs_image = rootfs_image)
    name_runner = mcreate_rule(name, runner, "", runner_cfg, kwargs, cfg)
