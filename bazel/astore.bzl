def _astore_upload(ctx):
  push = ctx.actions.declare_file("astore_upload.sh")

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
    "targets": attr.label_list(allow_files=True, providers=[DefaultInfo], mandatory=True),
    "dir": attr.string(
      doc = "All the targets outputs will be uploaded as different files in an astore directory.",
    ),
    "file": attr.string(
      doc = "All the targets outputs will be uploaded as the same file in an astore directory. " +
            "This is useful when you have multiple targets to build the same binary for different " +
            "architectures or operating systems.",
    ),
    "_astore_upload_file": attr.label(
      default = Label("//bazel:astore_upload_file.sh"),
      allow_single_file = True,
    ),
    "_astore_upload_dir": attr.label(
      default = Label("//bazel:astore_upload_dir.sh"),
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
