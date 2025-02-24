load("//bazel/linux:providers.bzl", "KernelBundleInfo", "KernelImageInfo", "KernelModulesInfo", "KernelTreeInfo", "RootfsImageInfo", "RuntimeBundleInfo")
load("//bazel/linux/platforms:defs.bzl", "kernel_aarch64_transition", "kernel_aarch64_constraints")
load("//bazel/linux:utils.bzl", "expand_deps", "get_compatible", "is_module")
load("//bazel/linux:test.bzl", "kunit_test")
load("//bazel/linux:uml.bzl", "kernel_uml_run")
load("//bazel/utils:messaging.bzl", "location", "package")
load("@bazel_skylib//lib:shell.bzl", "shell")
load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

def _kernel_modules(ctx):
    modules = ctx.attr.modules
    srcdir = ctx.file.makefile.dirname

    ki = ctx.attr.kernel[KernelTreeInfo]
    bundled = []

    inputs = ctx.files.srcs + ctx.files.kernel
    includes = []
    quote_includes = []
    extra_symbols = []

    kdeps = []
    for d in ctx.attr.kdeps:
        kdeps.extend(get_compatible(ctx, ki.arch, ki.package, d))

    for d in ctx.attr.deps:
        if not is_module(d):
            if CcInfo in d:
                inputs += d[CcInfo].compilation_context.headers.to_list()
                includes += d[CcInfo].compilation_context.includes.to_list()
                quote_includes += d[CcInfo].compilation_context.quote_includes.to_list()

            inputs.extend(d.files.to_list())
        else:
            mods = get_compatible(ctx, ki.arch, ki.package, d)

            kdeps.extend(mods)
            for mod in mods:
                extra_symbols.extend([f for f in mod.files if f.extension == "symvers"])
                inputs.extend(mod.files)

    outputs = []
    message = ""
    copy_command = ""
    for m in modules:
        message += "kernel: compiling %s for arch:%s kernel:%s" % (m, ki.arch, ki.package)

        # ctx.attr.name is $kernel-$original_bazel_rule_name
        outfile = "{kernel}/{name}/{arch}/{module}".format(
            kernel = ki.name,
            name = ctx.attr.name.removeprefix(ki.name + "-"),
            arch = ki.arch,
            module = m,
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
    tools = []
    if ki.arch != "host":
        arch = ki.arch
        # map aarch64 to arm64.  The kernel uses arm64, bazel uses aarch64. sigh.
        if arch == "aarch64":
            arch = "arm64"
        extra.append("ARCH=" + arch)

        toolchain = ctx.toolchains["@bazel_tools//tools/cpp:toolchain_type"].cc
        tools = toolchain.all_files

        # The compiler ends in "gcc", which we strip off to obtain the compiler prefix.
        compiler_prefix = toolchain.compiler_executable[:-3]
        extra.append("CROSS_COMPILE=$PWD/{}".format(compiler_prefix))

    if ctx.attr.extra:
        extra += ctx.attr.extra

    extra_symbols = " ".join(["$PWD/" + e.path for e in extra_symbols])

    if extra_symbols:
        extra.append("KBUILD_EXTRA_SYMBOLS=\"%s\"" % (extra_symbols))

    extra.append('BAZEL_BIN_DIR="%s"' % ctx.bin_dir.path)
    extra.append('BAZEL_GEN_DIR="%s"' % ctx.genfiles_dir.path)

    if ctx.attr.silent:
        silent = "-s"
    else:
        silent = ""

    if ctx.attr.jobs == -1:
        jobs = "-j$(nproc)"
    else:
        jobs = "-j%d" % ctx.attr.jobs

    kernel_build_dir = ctx.attr.local_kernel[BuildSettingInfo].value if ctx.attr.local_kernel and ki.arch == "host" else kernel_build_dir

    print(ctx.attr.local_kernel)
    print(kernel_build_dir)

    make_args = ctx.attr.make_format_str.format(
        src_dir = srcdir,
        kernel_build_dir = kernel_build_dir,
        modules = " ".join(modules),
    )

    #print(make_args)

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

    # Force GCC to produce colored output
    cflags += " -fdiagnostics-color=always"

    for include in depset(includes).to_list():
        cflags += " -I $PWD/%s" % include

    for include in depset(quote_includes).to_list():
        cflags += " -iquote $PWD/%s" % include

    extra.append("EXTRA_CFLAGS+=\"%s\"" % cflags)

    # equivalent to tags = ["no-remote"], but tags are not configurable attributes -
    # https://github.com/bazelbuild/bazel/issues/2971
    execution_requirements = None if ctx.attr.remote else {"no-remote": "foo"}
    print(execution_requirements)

    ctx.actions.run_shell(
        mnemonic = "KernelBuild",
        progress_message = message,
        command = "EXT_BUILD_ROOT=$PWD make {silent} {jobs} {make_args} {extra_args} && {copy_command}".format(
            silent = silent,
            jobs = jobs,
            make_args = make_args,
            extra_args = " ".join(extra),
            copy_command = copy_command,
        ),
        outputs = outputs,
        inputs = inputs,
        use_default_shell_env = True,
        tools = tools,
        execution_requirements = execution_requirements,
    )

    bundled.append(KernelModulesInfo(
        label = ctx.label,
        arch = ki.arch,
        package = ki.package,
        files = outputs,
        kdeps = kdeps,
        setup = ctx.attr.setup,
    ))

    return [
        DefaultInfo(
            files = depset(outputs),
            runfiles = ctx.runfiles(files = outputs),
        ),
        KernelBundleInfo(
            modules = bundled,
        ),
    ]

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
        "remote": attr.bool(
            default = True,
            doc = "Whether to allow remote execution, should be set to false when building against a local kernel",
        ),
        "local_kernel": attr.label(
            mandatory = False,
            providers = [BuildSettingInfo],
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
        "jobs": attr.int(
            default = -1,
            doc = "Sets the number of jobs to run simultaneously. The default is -1 which translates to the number of processing units available (nproc).",
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
    toolchains = [
        "@bazel_tools//tools/cpp:toolchain_type",
    ],
)

def _kernel_modules_bundle(ctx):
    modules = []
    runfiles = ctx.runfiles()
    for module in ctx.attr.modules:
        runfiles = runfiles.merge(module[DefaultInfo].default_runfiles)
        if KernelModulesInfo in module:
            modules.append(module)
        elif KernelBundleInfo in module:
            modules.extend(module[KernelBundleInfo].modules)

    return [
        DefaultInfo(
            files = depset(ctx.files.modules),
            runfiles = runfiles,
        ),
        KernelBundleInfo(
            modules = modules,
        ),
    ]

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

def _normalize_kernel(kernel_tree_label):
    """Ensures a kernel string points to a repository rule, with bazel @ syntax."""

    if not kernel_tree_label.startswith("@"):
        kernel_tree_label = "@" + kernel_tree_label

    # kernel_tree_string: a flat string that can be used to create other labels
    # The label might have '@', slashes ('/'), and colons (':'). Remove those.
    kernel_tree_string = kernel_tree_label.replace("@", "").replace("//:", "-").replace("/", "-")

    arch = "host"
    # this is janky, but if the label contains the string "arm64" or
    # "aarch64" force the architecture to aarch64.  Really at this
    # macro level we would like to peak inside the KernelTreeInfo
    # provider to learn the CPU architecture.
    if (kernel_tree_string.count("arm64") + kernel_tree_string.count("aarch64")) > 0:
        arch = "aarch64"

    return (kernel_tree_label, kernel_tree_string, arch)

# TODO: remove tags; see INFRA-1516
#
# Setting these tags prevents platform targets from being built by
# pre/postsubmit.  The targets are still built when non-tagged
# targets use a transition to build them.
PLATFORM_NO_BUILD_TAGS = [
    "manual",
    "no-postsubmit",
    "no-presubmit",
]

def _gen_module_rule(arch, *args, **kwargs):
    if arch == "host":
        kernel_modules_rule(*args, **kwargs)
        return

    if arch != "aarch64":
        fail("Unknown architecture: " + arch)

    kwargs["target_compatible_with"] =  kernel_aarch64_constraints

    # underlying module rule needs a different name as the top level
    # transition rule will use the original name.
    original = kwargs["name"]
    arch_module_name = original + "-" + arch
    kwargs["name"] = arch_module_name

    # Skip non-transitioned targets for CI
    original_tags = kwargs.get("tags", [])
    kwargs["tags"] = original_tags + PLATFORM_NO_BUILD_TAGS

    # put down original module rule
    kernel_modules_rule(*args, **kwargs)

    # Add a transition with the original name
    #
    # These target will be built by CI.
    kernel_aarch64_transition(
        name = original,
        target = ":" + arch_module_name,
        tags = original_tags,
        visibility = kwargs["visibility"],
    )

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
        (kernel_tree_label, unused, arch) = _normalize_kernel(kernels.pop())
        kwargs["kernel"] = kernel_tree_label
        _gen_module_rule(arch, *args, **kwargs)
        return

    targets = []
    original = kwargs["name"]
    for kernel in kernels:
        (kernel_tree_label, kernel_tree_string, arch) = _normalize_kernel(kernel)
        name = kernel_tree_string + "-" + original
        targets.append(":" + name)

        kwargs["name"] = name
        kwargs["kernel"] = kernel_tree_label
        _gen_module_rule(arch, *args, **kwargs)

    # This creates a target with the name chosen by the user that
    # builds all the modules for all the requested kernels at once.
    # Without this, the user can only build :all, or the specific
    # module for a specific kernel.
    return kernel_modules_bundle(name = original, modules = targets, visibility = kwargs.get("visibility"))

def kernel_module(*args, **kwargs):
    """Convenience wrapper around kernel_modules_rule.

    Use this wrapper for building a single out of tree kernel module.

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
      kernel: a kernel tree, indicating the kernel source tree, CPU architecture,
            and kernel configuration to build the module against.
      kernels: list of kernel trees (same as above). kernel_module will instantiate multiple
            kernel_module_rule, one per kernel spec, and ensure they all build in parallel.
    """

    if "makefile" not in kwargs:
        kwargs["makefile"] = "Makefile"

    if "srcs" not in kwargs:
        include = ["**/*.c", "**/*.h", kwargs["makefile"]]
        kwargs["srcs"] = native.glob(include = include, exclude = BUILD_LEFTOVERS, allow_empty = False)

    kwargs["modules"] = [kwargs.pop("module", kwargs["name"])]

    if "make_format_str" not in kwargs:
        kwargs["make_format_str"] = "-C {kernel_build_dir} M=$PWD/{src_dir} {modules}"

    kwargs["local_kernel"] = select({
        "@enfabrica//:kernel_dir_placeholder": None,
        "//conditions:default": "@enfabrica//:kernel_dir",
    })

    kwargs["remote"] = select({
        "@enfabrica//:kernel_dir_placeholder": True,
        "//conditions:default": False,
    })

    return _kernel_module_targets(*args, **kwargs)

def _kernel_tree(ctx):
    return [DefaultInfo(files = depset(ctx.files.files)), KernelTreeInfo(
        name = ctx.attr.name,
        package = ctx.attr.package,
        arch = ctx.attr.arch,
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
        "arch": attr.string(
            doc = "Architecture this tree was built for. Will only accept moudules for this arch.",
            default = "host",
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

# Existing kernel_test() rule consumers assume the runner is UML.
#
# This defines kernel_test() in terms of kunit_test(), which uses the
# QEMU runner by default.
#
# Remove this once all kernel_test() consumers are converted to use
# kunit_test() directly.

def kernel_test(name, kernel_image, module, **kwargs):
    """[Deprecated] Defines a UML based kunit test.

    Args:
      name: test name
      kernel_image: label, something like @type-of-kernel//:image,
          a kernel image to use.
      module: label, a module representing a kunit test to run.
      kwargs: options common to all instantiated rules.

    Example:

      kernel_test(
          name = "a_uml_kunit_test",
          kernel_image = "@testing-latest-kernel//:image",
          module = ":uml_kunit_test",
      )

    [Deprecated]: New tests should use the kunit_test() rule instead.
    """

    # Use the UML runner for the legacy tests
    kunit_test(name, kernel_image, module, runner = kernel_uml_run, **kwargs)
