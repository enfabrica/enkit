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
    if ctx.attr.dir and ctx.attr.file:
        fail("in '%s' rule for an astore_upload in %s - you can only set dir or file, not both" % (ctx.attr.name, ctx.build_file_path), "dir")

    files = [ctx.executable._astore_client]
    targets = []
    for target in ctx.attr.targets:
        targets.extend([t.short_path for t in target.files.to_list()])
        files.extend([f for f in target.files.to_list()])

    template = ctx.file._astore_upload_file
    if ctx.attr.dir:
        template = ctx.file._astore_upload_dir

    uidfile = ""
    if ctx.attr.uidfile:
        uidfile = ctx.files.uidfile[0].short_path
        files.append(ctx.files.uidfile[0])

    ctx.actions.expand_template(
        template = template,
        output = ctx.outputs.executable,
        substitutions = {
            "{astore}": ctx.executable._astore_client.short_path,
            "{targets}": " ".join(targets),
            "{file}": ctx.attr.file,
            "{dir}": ctx.attr.dir,
            "{uidfile}": uidfile,
        },
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = files)
    return [DefaultInfo(runfiles = runfiles)]

astore_upload = rule(
    implementation = _astore_upload,
    attrs = {
        "targets": attr.label_list(
            allow_files = True,
            providers = [DefaultInfo],
            mandatory = True,
            cfg = "target",
        ),
        "dir": attr.string(
            doc = "All the targets outputs will be uploaded as different files in an astore directory.",
        ),
        "file": attr.string(
            doc = "All the targets outputs will be uploaded as the same file in an astore directory. " +
                  "This is useful when you have multiple targets to build the same binary for different " +
                  "architectures or operating systems.",
        ),
        "uidfile": attr.label(
            allow_files = True,
            providers = [DefaultInfo],
            mandatory = False,
            doc = "If specified, will attempt to update the UID variable in this (build) file.",
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
            default = Label("@net_enfabrica_binary_astore//file"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
    executable = True,
    doc = """Uploads artifacts to an artifact store - astore.

With this rule, you can easily upload the output of a build rule
to an artifact store.

Optionally, this rule can update a BUILD file (or other text file) to contain
the generated UID for each uploaded target.  This functionality is enabled
by specifying the "uidfile" attribute.

The script will search that file for a line matching:

UID_TARGETNAME = "some-uid-string"

And update "some-uid-string" with the UID of the file that was just uploaded.

The variable name "UID_TARGETNAME" is formed by transforming the base name
of the target in the following manner:

  - all non-alphanumeric characters are replaced with underscores.
  - all alphabetic characters are converted to uppercase.
  - "UID_" is prepended.

For example: a target named foo/bar:some-script.sh would correspond with the
UID variable name "UID_SOME_SCRIPT_SH".

Note that the "uidfile" functionality is currently only supported when using
the "file" attribute, but not the "dir" attribute.

TODO(jonathan): add support for the "dir" attribute.
""",
)

def _astore_download(ctx):
    if ctx.attr.output:
        output = ctx.outputs.output
    else:
        output = ctx.actions.declare_file(ctx.attr.download_src.split("/")[-1])
    command = ("%s download --no-progress --overwrite -o %s" %
               (ctx.executable._astore_client.path, output.path))
    execution_requirements = {
        # We can't run these remotely since remote workers won't have
        # credentials to fetch from astore.
        "requires-network": "Downloads from astore",
        "timeout": "%d" % ctx.attr.timeout,
    }
    if ctx.attr.arch:
        command += " -a " + ctx.attr.arch
    if ctx.attr.uid:
        command += " --force-uid %s" % ctx.attr.uid
    else:
        command += " --tag %s %s" % (ctx.attr.astore_tag, ctx.attr.download_src)
        execution_requirements["no-cache"] = "Not hermetic, since uid was not specified."
        execution_requirements["no-remote"] = "Not hermetic, since uid was not specified."
        # TODO(ccontavalli): an old comment claimed the following, is it
        # still true?
        # # We should also avoid remotely caching since:
        # # * this means we need to give individuals permissions to remotely
        # #   cache local actions, which we currently don't do
        # # * we might spend lots of disk/network caching astore artifacts
        # #   remotely

    if ctx.attr.digest:
        command += " && (echo \"%s\" %s | sha256sum --check -)" % (ctx.attr.digest, output.path)
    ctx.actions.run_shell(
        command = command,
        tools = [ctx.executable._astore_client],
        outputs = [output],
        execution_requirements = execution_requirements,
        use_default_shell_env = True,
    )
    return [DefaultInfo(
        files = depset([output]),
        runfiles = ctx.runfiles([output]),
    )]

# TODO: add an optional "uid" attribute to this rule
# TODO: add an optional "digest" attribute to this rule
astore_download = rule(
    implementation = _astore_download,
    attrs = {
        "download_src": attr.string(
            doc = "Provided the full path, download a file from astore.",
            mandatory = True,
        ),
        "astore_tag": attr.string(
            doc = "Astore tag name to specify version of the artifact to download",
            default = "latest",
        ),
        "arch": attr.string(
            doc = "Architecture to download the file for.",
        ),
        "timeout": attr.int(
            doc = "Timeout for astore download operation, in seconds.",
            default = 10 * 60,
        ),
        "output": attr.output(
            doc = "Overrides the default output path, if used.",
        ),
        "uid": attr.string(
            doc = "The UID of a specific version of the file to download.",
            mandatory = False,
            default = "",
        ),
        "digest": attr.string(
            doc = "The sha256 digest of the file that we expect to receive.",
            mandatory = False,
            default = "",
        ),
        "_astore_client": attr.label(
            default = Label("@net_enfabrica_binary_astore//file"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
    doc = """Downloads artifacts from artifact store - astore.

With this rule, you can easily download
files from an artifact store.""",
)

def _astore_download_and_verify(rctx, dest, uid, digest, timeout):
    # Download by UID to destination
    enkit_args = [
        "enkit",
        "astore",
        "download",
        "--force-uid",
        uid,
        "--output",
        dest,
        "--overwrite",
    ]
    res = rctx.execute(enkit_args, timeout = timeout)
    if res.return_code:
        fail("Astore download failed\nArgs: {}\nStdout:\n{}\nStderr:\n{}\n".format(
            enkit_args,
            res.stdout,
            res.stderr,
        ))

    # Check digest of downloaded file
    checksum_args = ["sha256sum", dest]
    res = rctx.execute(checksum_args, timeout = 60)
    if res.return_code:
        fail("Failed to calculate checksum\nArgs: {}\nStdout:\n{}\nStderr:\n{}\n".format(
            checksum_args,
            res.stdout,
            res.stderr,
        ))

    got_digest = res.stdout.strip().split(" ")[0]
    if got_digest != digest:
        fail("WORKSPACE repository {}: Got digest {}; expected digest {}".format(
            rctx.attr.name,
            got_digest,
            digest,
        ))

def astore_download_and_extract(ctx, digest, stripPrefix, path = None, uid = None, timeout = 10 * 60):
    """Fetch and extract a package from astore.

    This method downloads a package stored as an archive in astore, verifies
    the sha256 digest provided by calling rules, and strips out any archive path
    components provided by the caller. This function is only meant to be used by
    Bazel repository rules and they do not maintain a dependency graph and the
    ctx object is different than the ones used with regular rules.
    """

    # Hard to rename this var, since downstream calls this function using
    # keyword args, naming ctx explicitly. However, it is a repository context,
    # so use rctx throughout to minimize confusion.
    rctx = ctx

    f = rctx.path((path or rctx.attr.path).split("/")[-1])
    uid = uid or rctx.attr.uid
    if hasattr(rctx.attr, "timeout"):
        timeout = rctx.attr.timeout

    _astore_download_and_verify(rctx, f, uid, digest, timeout)

    rctx.extract(
        archive = f,
        output = ".",
        stripPrefix = stripPrefix,
    )
    rctx.delete(f)

    if hasattr(rctx.attr, "build") and rctx.attr.build:
        rctx.template("BUILD.bazel", rctx.attr.build)

# This wrapper is in place to allow a rolling upgrade across Enkit and the
# external repositories which consume the kernel_tree_version rule defined in
# //enkit/linux/defs.bzl, which uses "sha256" as the attribute name instead of
# "digest".
def _astore_download_and_extract_impl(rctx):
    astore_download_and_extract(rctx, rctx.attr.digest, rctx.attr.strip_prefix)

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

def _astore_file_impl(rctx):
    f = rctx.path(rctx.attr.path.split("/")[-1])

    _astore_download_and_verify(rctx, f, rctx.attr.uid, rctx.attr.digest, 10 * 60)

    # Fix permissions on downloaded file.
    #
    # Executable bit is not preserved on round-trip through astore, so this is
    # passed in via a rule attribute. Otherwise, permissions are the two that
    # git typically supports.
    perms = "0644"
    if rctx.attr.executable:
        perms = "0755"
    rctx.execute(["chmod", perms, f])

    # Create a WORKSPACE file
    rctx.file("WORKSPACE", content = "", executable = False)

    # Create a BUILD file
    rctx.file("BUILD.bazel", content = 'exports_files(glob(["**/*"]))', executable = False)

astore_file = repository_rule(
    implementation = _astore_file_impl,
    attrs = {
        "path": attr.string(
            doc = "Path to the object in astore",
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
        "executable": attr.bool(
            doc = "Whether this file is an executable",
            default = False,
        ),
    },
)
