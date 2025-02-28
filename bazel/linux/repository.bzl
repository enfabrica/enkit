load("//bazel/astore:defs.bzl", "astore_download_and_extract")
load("//bazel/utils:macro.bzl", "mconfig")
load("//bazel/utils:messaging.bzl", "package")

def _package_to_distro_version(package):
    """Converts a package name to a distro id and kernel version.

    To uniquely identify a kernel version, the enkit rules use a
    'package' string made by a 'distro' identifier, followed
    by '-', followed by the kernel version.

    For example: 'debian-5.11.0-rc6' or 'ubuntu-5.11.0-rc6' or
    'enf-5.13.0-19-1-1651796444-gffc1f1c68bba-generic'.

    This is necessary as the kernel version specified must match that
    used by the kernel in the .tar.gz supplied. For example, it must
    match the modules directory name /lib/modules/<version>/ or the
    vmlinuz-<version> name. As such, it cannot be easily changed.

    At the same time, it is not guaranteed to be unique, depending
    on the build system used, and the origin of the kernel, we can
    have two kernels '5.11.0-rc6' not really compatible with one
    another.

    The distro id allows to create a namespace, where each kernel
    version identifier is guaranteed to be unique (different kernel
    variants would have different version ids or different distro).

    Given a package name, this function splits it into a
    'distro' identifier, followed by a 'version' number.
    """
    distro, version = package.split("-", 1)
    return distro, version

def _install_kernel_tree(ctx, required):
    _, version = _package_to_distro_version(ctx.attr.package)

    # Check if the package contains a kernel tree.
    install_script = "install-" + version + ".sh"
    install_script_path = ctx.path(install_script)
    separator = "========================"
    if not install_script_path.exists:
        if required:
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
        return None

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

    return ("tree", ctx.attr.template_tree, {
        "build_path": "%s/build" % (result.stdout.strip()),
    })

def _install_kernel_modules(ctx, required, depmod = True):
    _, version = _package_to_distro_version(ctx.attr.package)

    modules_path = "lib/modules/{version}".format(version = version)
    modules_builtin = ctx.path(modules_path + "/modules.builtin")
    if not modules_builtin.exists:
        if required:
            fail("While looking for modules in {name} - could not find {path}. Incorrect package?".format(
                name = ctx.name,
                path = modules_builtin,
            ))
        return None

    if depmod:
        cmd = "depmod -a -b \"$(realpath .)\" {version}".format(version = version)
        result = ctx.execute(["/bin/sh", "-c", cmd])
        if result.return_code != 0:
            fail("Running command {cmd} failed with status {status}:\n{stdout}\n{stderr}".format(
                cmd = cmd,
                status = result.return_code,
                stdout = result.stdout,
                stderr = result.stderr,
            ))

    return ("modules", ctx.attr.template_modules, {
        "modules_path": modules_path,
    })

def _install_kernel_image(ctx, candidate, required):
    _, version = _package_to_distro_version(ctx.attr.package)

    versioned = "boot/vmlinuz-{version}".format(version = version)
    versioned_path = ctx.path(versioned)
    candidate_path = ctx.path(candidate)
    if not versioned_path.exists and not candidate_path.exists:
        if required:
            fail("While looking for kernel in {name} - could not find {cpath} nor {vpath}. Incorrect package?".format(
                name = ctx.name,
                cpath = candidate_path,
                vpath = versioned_path,
            ))
        return None

    path = versioned
    if not versioned_path.exists:
        path = candidate

    # vmlinuz file may need to be executed (for uml, mark it as such).
    ctx.execute(["chmod", "0755", path])
    return ("image", ctx.attr.template_image, {
        "image_path": path,
    })

def _local_kernel_impl(repository_ctx):
    if repository_ctx.attr.path:
        target = repository_ctx.attr.path
    else:
        # By default we'll assume a standard install location
        # on the users host.
        target = repository_ctx.os.environ['HOME'] + "/rootfs"

    # create a symlink into the external repo dir
    repository_ctx.symlink(
        target,
        "",
    )

    substitutions = {
        "{version}": repository_ctx.attr.version,
    }

    repository_ctx.template(
        "BUILD.bazel",
        repository_ctx.attr._build_tpl,
        substitutions,
    )

local_kernel_package = repository_rule(
    doc = "Imports a local kernel install into the bazel workspace",
    implementation = _local_kernel_impl,
    attrs = {
        "path": attr.string(
            doc = "Optional path to kernel install directory. Defaults to /home/{user}/rootfs",
        ),
        "version": attr.string(
            doc = "Version metadata for kernel image - will be appended to vmlinuz-{version}",
            mandatory = True,
        ),
        "_build_tpl": attr.label(
            doc = "BUILD template for kernel_image target and kernel_modules filegroup",
            default = "//bazel/linux:BUILD.bazel.tpl",
        ),
    },
)
def _kernel_package(ctx):
    if ctx.attr.url and not (ctx.attr.path or ctx.attr.uid):
        if ctx.attr.extract:
            ctx.download_and_extract(ctx.attr.url, output = ".", sha256 = ctx.attr.sha256, auth = ctx.attr.auth, stripPrefix = ctx.attr.strip_prefix)
        else:
            # Without extraction, privileges are not preserved. Mark the file as executable so that kernel images can run.
            ctx.download(ctx.attr.url, executable = True, output = ctx.attr.package, sha256 = ctx.attr.sha256, auth = ctx.attr.auth)

    elif (ctx.attr.path and ctx.attr.uid) and not ctx.attr.url and ctx.attr.extract:
        astore_download_and_extract(
            ctx,
            path = ctx.attr.path,
            uid = ctx.attr.uid,
            digest = ctx.attr.sha256,
            stripPrefix = ctx.attr.strip_prefix,
        )
    else:
        fail("WORKSPACE repository {name}: Either provide an 'url' attribute for HTTP download, OR an astore path and uid (only if extract = True)".format(name = ctx.attr.name))

    fragments = []
    if "tree" in ctx.attr.allowed:
        fragments.append(_install_kernel_tree(ctx, "tree" in ctx.attr.required))
    if "image" in ctx.attr.allowed:
        fragments.append(_install_kernel_image(ctx, ctx.attr.package, "image" in ctx.attr.required))
    if "modules" in ctx.attr.allowed:
        fragments.append(_install_kernel_modules(ctx, "modules" in ctx.attr.required, depmod = ctx.attr.depmod))

    common = {
        "name": ctx.name,
        "package": ctx.attr.package,
        "arch": ctx.attr.arch,
        "utils": str(ctx.attr._utils),
    }
    for frag in fragments:
        if not frag:
            continue
        _, _, subs = frag
        common.update(subs)

    buildfile = []
    for frag in fragments:
        if not frag:
            continue

        kind, template, _ = frag
        name = ctx.attr.names[kind].format(**common)
        subs = dict(common, name = name)

        data = ctx.read(template)
        buildfile.append("\n# Generated from " + package(template))
        buildfile.append(data.format(**subs))

    ctx.file("BUILD.bazel", content = "\n".join(buildfile))

kernel_package = repository_rule(
    doc = """Imports a file containing either a kernel tree, a kernel image, or its modules.""",
    implementation = _kernel_package,
    local = False,
    attrs = {
        "extract": attr.bool(
            doc = "Set to False if the downloaded file should not be unpacked (it is not a .tar.gz, .zip, ...).",
            default = True,
        ),
        "depmod": attr.bool(
            doc = "Set to False to disable running depmod when installing kernel modules.",
            default = True,
        ),
        "package": attr.string(
            doc = "The name of the downloaded image. Usually the format is 'distribution-kernel_version-arch', like custom-5.9.0-um.",
            mandatory = True,
        ),
        "arch": attr.string(
            doc = "The architecture this image was built for. 'host' means the architecture of the current machine.",
            default = "host",
        ),
        "sha256": attr.string(
            doc = "The sha256 of the downloaded package file.",
        ),
        "strip_prefix": attr.string(
            doc = "When unpacking a downloaded artifact, the directory prefix to remove in order to find other specified paths.",
        ),
        "url": attr.string(
            doc = "The url to download the kernel executable image from.",
        ),
        "auth": attr.string_dict(
            doc = "An auth dict as documented for the download_and_extract context rule as is.",
        ),
        "path": attr.string(
            doc = "Path to the object in astore.",
        ),
        "uid": attr.string(
            doc = "Astore UID of the desired version of the object.",
        ),
        "timeout": attr.int(
            doc = "Timeout to apply when downloading from astore, in seconds.",
            default = 10 * 60,
        ),
        "required": attr.string_list(
            doc = """\
Sets of components that must be provided in the package.
If those components cannot be found, the package is invalid.

This helps in detecting problems early, and in avoiding cryptic
messages in rules actually using the content.

Valid values are: image, modules, tree.""",
            default = ["image", "modules"],
        ),
        "allowed": attr.string_list(
            doc = """\
Sets of components that the package is allowed to provide.
Other components in the package are ignored.

This is useful when using bits and pieces from different
tarballs, to avoid undesired components from being picke dup.""",
            default = ["tree", "image", "modules"],
        ),
        "names": attr.string_dict(
            doc = """\
The name to give to each generated target.

For example, setting {"tree": "source", "modules": "modules"} means that the
source code in the linux tree will be reachable as @name-of-rule//:source,
while the modules will be reachable as @name-of-rule//:modules.

This is only used for backward compatibility with past
rules, recommend not changing this.""",
            default = {"tree": "source", "image": "image", "modules": "modules"},
        ),
        "template_image": attr.label(
            doc = "BUILD.bazel template to use for kernel images.",
            default = Label("//bazel/linux:templates/kernel_image.BUILD.bzl"),
            allow_single_file = True,
        ),
        "template_tree": attr.label(
            doc = "BUILD.bazel template to use for kernel trees.",
            default = Label("//bazel/linux:templates/kernel_tree.BUILD.bzl"),
            allow_single_file = True,
        ),
        "template_modules": attr.label(
            doc = "BUILD.bazel template to use for kernel modules.",
            default = Label("//bazel/linux:templates/kernel_modules.BUILD.bzl"),
            allow_single_file = True,
        ),
        "_utils": attr.label(
            default = Label("//bazel/linux:defs.bzl"),
            allow_single_file = True,
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

def kernel_tree_version(**kwargs):
    """Imports a specific kernel version to build out of tree modules.

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
    """
    kernel_package(**mconfig(kwargs, names = {"tree": "{name}"}, required = ["tree"], allowed = ["tree"]))

def kernel_image_version(**kwargs):
    """Imports a specific kernel executable image version to be used for kernel tests.

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
    """
    kernel_package(**mconfig(kwargs, extract = False, names = {"image": "{name}"}, required = ["image"], allowed = ["image"]))
