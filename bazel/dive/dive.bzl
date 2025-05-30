def _oci_dive_impl(ctx):
    tarball = ctx.file.src
    dive_bin = ctx.file.dive_bin

    exe = ctx.actions.declare_file(ctx.label.name + ".sh")

    ctx.actions.expand_template(
        template = ctx.file._dive_template,
        output = exe,
        substitutions = {
            "{{dive_bin}}": dive_bin.path,
            "{{tarball}}": tarball.short_path,
        },
        is_executable = True,
    )

    runfiles = [tarball, dive_bin]

    return [
        DefaultInfo(files = depset([exe]), runfiles = ctx.runfiles(files = runfiles), executable = exe),
    ]

oci_dive = rule(
    implementation = _oci_dive_impl,
    attrs = {
        "src": attr.label(
            mandatory = True,
            allow_single_file = [".tar"],
        ),
        "dive_bin": attr.label(
            default = "@dive_x86_64",
            allow_single_file = True,
        ),
        "_dive_template": attr.label(
            default = Label("//bazel/dive:dive.sh.tpl"),
            allow_single_file = True,
        ),
    },
    doc = """
Analyze an OCI tar archive with dive
""",
    executable = True,
)
