load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

AstoreMetadataProvider = provider(fields = ["tags"])

def _astore_tag_impl(ctx):
    # TODO(minor-fixes): If any validation is necessary on astore tag values,
    # logic can go here to fail the build on illegal values.
    return AstoreMetadataProvider(tags = ctx.build_setting_value)

astore_tag = rule(
    implementation = _astore_tag_impl,
    build_setting = config.string_list(flag = True, repeatable = True),
)

def _astore_url(package, uid, access_mod = "g", instance = "https://astore.corp.enfabrica.net"):
    """Returns a URL for a particular package version from astore."""
    if not package:
        fail("package not passed to astore_url substitution")

    if not uid:
        fail("uid not passed to astore_url substitution")

    if not package.startswith("/"):
        package = "/" + package
    return "{}/{}{}?u={}".format(
        instance,
        access_mod,
        package,
        uid,
    )

def astore_url(package, uid, instance  = "https://astore.corp.enfabrica.net"):
    _astore_url(package, uid, "d", instance)

def _get_url_and_sha256(kwargs):
    url = kwargs.pop("url", None)
    if not url:
        url = _astore_url(
            kwargs.pop("path", None),
            kwargs.pop("uid", None),
        )

    sha256 = kwargs.pop("sha256", None)
    if not sha256:
        sha256 = kwargs.pop("digest", None)

    return url, sha256

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

    tags = []
    if ctx.attr.upload_tag:
        tags.append(ctx.attr.upload_tag)
    tags.extend(ctx.attr._cmdline_upload_tag[AstoreMetadataProvider].tags)
    upload_tag = " ".join(["--tag={}".format(tag) for tag in tags])

    ctx.actions.expand_template(
        template = template,
        output = ctx.outputs.executable,
        substitutions = {
            "{astore}": ctx.executable._astore_client.short_path,
            "{targets}": " ".join(targets),
            "{file}": ctx.attr.file,
            "{dir}": ctx.attr.dir,
            "{uidfile}": uidfile,
            "{upload_tag}": upload_tag,
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
        "upload_tag": attr.string(
            doc = "Apply optional tag to binary during upload.",
        ),
        "_cmdline_upload_tag": attr.label(
            providers = [[AstoreMetadataProvider]],
            default = "//f/astore:upload_tag",
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

    sha_command = ":"
    if ctx.attr.digest:
        sha_command = "echo \"{digest}\" {path} | sha256sum --check -".format(digest = ctx.attr.digest, path = output.path)

    to_run = """\
set -uo pipefail
for attempt in $(seq {attempts}); do
  {command} && {{
    {sha_command} || {{
      echo "invalid SHA - rejected package - use sha256sum and update the 'digest' attribute" 1>&2
      echo "Download command: '{command}'" 1>&2
      exit 2
    }}
    exit 0
  }}

  echo "= Attempt #$attempt to run '{command}' failed - retrying in {sleep}s" 1>&2
  sleep {sleep}
done

echo "===================================================" 1>&2
echo "ERROR: Could not successfully complete astore download in {attempts} attempts - giving up" 1>&2
echo "Scroll up to see the problems." 1>&2
exit 1
""".format(command = command, sha_command = sha_command, sleep = ctx.attr.sleep, attempts = ctx.attr.attempts)

    ctx.actions.run_shell(
        command = to_run,
        tools = [ctx.executable._astore_client],
        outputs = [output],
        execution_requirements = execution_requirements,
        use_default_shell_env = True,
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
        "astore_tag": attr.string(
            doc = "Astore tag name to specify version of the artifact to download",
            default = "latest",
        ),
        "arch": attr.string(
            doc = "Architecture to download the file for.",
        ),
        "attempts": attr.int(
            doc = "If the download fails, retry up to this many times.",
            default = 10,
        ),
        "sleep": attr.int(
            doc = "In between failed attempts, wait this long before retrying, in seconds.",
            default = 1,
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
            cfg = "exec",
        ),
    },
    doc = """Downloads artifacts from artifact store - astore.

With this rule, you can easily download
files from an artifact store.""",
)

def astore_download_and_extract(repository_ctx, **kwargs):
    """Just a proxy to rctx.download_and_extract.

    Args:
        repository_ctx: the context of the repository rule
        **kwargs: arguments the same as for `repository_ctx.download_and_extract`,
        but also taking `path` and `uid` into account to substitute astore url,
        and considering 'digest' as alias to `sha256`.
    """

    url, sha256 = _get_url_and_sha256(kwargs)

    repository_ctx.download_and_extract(
        url = url,
        sha256 = sha256,
        **kwargs,
    )

def astore_package(**kwargs):
    """Just a proxy to http_archive.

    Args:
        **kwargs: arguments the same as for `http_archive`, but also
        taking `path` and `uid` into account to substitute astore url,
        and considering 'digest' as alias to `sha256`, `build` to `build_file`.
    """
    
    url, sha256 = _get_url_and_sha256(kwargs)

    build_file = kwargs.pop("build", None)
    if not build_file:
        build_file = kwargs.pop("build_file", None)

    http_archive(
        url = url,
        sha256 = sha256,
        build_file = build_file,
        **kwargs,
    )

def _astore_file_impl(rctx):
    output = rctx.path(rctx.attr.path.split("/")[-1])
    url = _astore_url(rctx.attr.path, rctx.attr.uid)
    rctx.download(
        url = url,
        output = output,
        sha256 = rctx.attr.digest,
        executable = rctx.attr.executable,
    )

    rctx.file("BUILD.bazel", content = 'exports_files(glob(["**/*"]))')

astore_file = repository_rule(
    doc = "Downloads a file from astore without unpacking, provides it exports_files.",
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
