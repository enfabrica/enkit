def astore_url(package, uid, instance = "https://astore.corp.enfabrica.net"):
    """Returns a URL for a particular package version from astore."""
    if not package.startswith("/"):
        package = "/" + package
    return "{}/d{}?u={}".format(
        instance,
        package,
        uid,
    )

def _astore_upload(ctx):
    push = ctx.actions.declare_file("{}.sh".format(ctx.attr.name))

    if ctx.attr.dir and ctx.attr.file:
        fail("in '%s' rule for an astore_upload in %s - you can only set dir or file, not both" % (ctx.attr.name, ctx.build_file_path), "dir")

    inputs = [ctx.executable._astore_client]
    targets = []
    for target in ctx.attr.targets:
        targets.extend([t.short_path for t in target.files.to_list()])
        inputs.extend([f for f in target.files.to_list()])

    template = ctx.file._astore_upload_file
    if ctx.attr.dir:
        template = ctx.file._astore_upload_dir

    ctx.actions.expand_template(
        template = template,
        output = push,
        substitutions = {
            "{astore}": ctx.executable._astore_client.short_path,
            "{targets}": " ".join(targets),
            "{file}": ctx.attr.file,
            "{dir}": ctx.attr.dir,
        },
        is_executable = True,
    )
    return [DefaultInfo(executable = push, runfiles = ctx.runfiles(inputs))]

astore_upload = rule(
    implementation = _astore_upload,
    attrs = {
        "targets": attr.label_list(allow_files = True, providers = [DefaultInfo], mandatory = True),
        "dir": attr.string(
            doc = "All the targets outputs will be uploaded as different files in an astore directory.",
        ),
        "file": attr.string(
            doc = "All the targets outputs will be uploaded as the same file in an astore directory. " +
                  "This is useful when you have multiple targets to build the same binary for different " +
                  "architectures or operating systems.",
        ),
        "_astore_upload_file": attr.label(
            default = Label("//bazel/astore:astore_upload_file.sh"),
            allow_single_file = True,
        ),
        "_astore_upload_dir": attr.label(
            default = Label("//bazel/astore:astore_upload_dir.sh"),
            allow_single_file = True,
        ),
        "_astore_client": attr.label(
            default = Label("//astore/client:astore"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
    executable = True,
    doc = """Uploads artifacts to an artifact store - astore.

With this rule, you can easily upload the output of a build rule
to an artifact store.""",
)

def _astore_download(ctx):
    output = ctx.actions.declare_file(ctx.attr.download_src.split("/")[-1])
    command = ("%s download --no-progress -o %s %s" %
               (ctx.executable._astore_client.path, output.path, ctx.attr.download_src))
    if ctx.attr.arch:
        command += " -a " + ctx.attr.arch
    ctx.actions.run_shell(
        command = command,
        tools = [ctx.executable._astore_client],
        outputs = [output],
        execution_requirements = {
            # We can't run these remotely since remote workers won't have
            # credentials to fetch from astore.
            # We should also avoid remotely caching since:
            # * this means we need to give individuals permissions to remotely
            #   cache local actions, which we currently don't do
            # * we might spend lots of disk/network caching astore artifacts
            #   remotely
            "no-remote": "Don't run remotely or cache remotely",
            "requires-network": "Downloads from astore",
            "no-cache": "Not hermetic, since it doesn't refer to packages by hash",
            "timeout": "%d" % ctx.attr.timeout,
        },
    )
    return [DefaultInfo(
       files = depset([output]),
       runfiles = ctx.runfiles([output]),
    )]

astore_download = rule(
    implementation = _astore_download,
    attrs = {
        "download_src": attr.string(
            doc = "Provided the full path, download a file from astore.",
            mandatory = True,
        ),
        "arch": attr.string(
            doc = "Architecture to download the file for.",
        ),
        "timeout": attr.int(
            doc = "Timeout for astore download operation, in seconds.",
            default = 10*60,
        ),
        "_astore_client": attr.label(
            default = Label("//astore/client:astore"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
    doc = """Downloads artifacts from artifact store - astore.

With this rule, you can easily download
files from an artifact store.""",
)

def astore_download_and_extract(ctx, digest, stripPrefix, path = None, uid = None, timeout = 10 * 60):
    """Fetch and extract a package from astore.

    This method downloads a package stored as an archive in astore, verifies
    the sha256 digest provided by calling rules, and strips out any archive path
    components provided by the caller. This function is only meant to be used by
    Bazel repository rules and they do not maintain a dependency graph and the
    ctx object is different than the ones used with regular rules.
    """
    f = ctx.path((path or ctx.attr.path).split("/")[-1])

    # Download archive
    enkit_args = [
        "enkit",
        "astore",
        "download",
        "--force-uid",
        uid or ctx.attr.uid,
        "--output",
        f,
        "--overwrite",
    ]
    if "timeout" in ctx.attr:
      timeout = ctx.attr.timeout
    res = ctx.execute(enkit_args, timeout = timeout)
    if res.return_code:
        fail("Astore download failed\nArgs: {}\nStdout:\n{}\nStderr:\n{}\n".format(
            enkit_args,
            res.stdout,
            res.stderr,
        ))

    # Check digest of archive
    checksum_args = ["sha256sum", f]
    res = ctx.execute(checksum_args, timeout = 60)
    if res.return_code:
        fail("Failed to calculate checksum\nArgs: {}\nStdout:\n{}\nStderr:\n{}\n".format(
            checksum_args,
            res.stdout,
            res.stderr,
        ))

    got_digest = res.stdout.strip().split(" ")[0]
    if got_digest != digest:
        fail("WORKSPACE repository {}: Got digest {}; expected digest {}".format(
            ctx.attr.name,
            got_digest,
            digest,
        ))

    ctx.extract(
        archive = f,
        output = ".",
        stripPrefix = stripPrefix,
    )
    ctx.delete(f)

    if hasattr(ctx.attr, "build") and ctx.attr.build:
        ctx.template("BUILD.bazel", ctx.attr.build)

# This wrapper is in place to allow a rolling upgrade across Enkit and the
# external repositories which consume the kernel_tree_version rule defined in
# //enkit/linux/defs.bzl, which uses "sha256" as the attribute name instead of
# "digest".
def _astore_download_and_extract_impl(ctx):
    astore_download_and_extract(ctx, ctx.attr.digest, ctx.attr.strip_prefix)

astore_package = repository_rule(
    implementation = _astore_download_and_extract_impl,
    attrs = {
        "build": attr.label(
            doc = "An optional BUILD file to copy in the unpacked tree.",
            allow_single_file = True,
        ),
        "path": attr.string(
            doc = "Path to the object in astore.",
            mandatory = True,
        ),
        "uid": attr.string(
            doc = "Astore UID of the desired version of the object.",
            mandatory = True,
        ),
        "digest": attr.string(
            doc = "SHA256 digest of the object.",
            mandatory = True,
        ),
        "strip_prefix": attr.string(
            doc = "Optional path prefix to strip out of the archive.",
        ),
        "timeout": attr.int(
            doc = "Timeout for astore fetch operation, in seconds.",
            default = 10 * 60,
        ),
    },
)
