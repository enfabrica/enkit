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
