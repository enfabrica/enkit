def _json_to_yaml_impl(ctx):
    output = ctx.actions.declare_file(ctx.attr.output)
    ctx.actions.run_shell(
        command = "{} -P -o yaml {} > {}".format(ctx.executable._tool.path, ctx.file.src.path, output.path),
        progress_message = "Converting {} to yaml".format(ctx.file.src.basename),
        inputs = [ctx.file.src],
        tools = [ctx.executable._tool],
        outputs = [output],
    )
    return [DefaultInfo(files = depset([output]))]

json_to_yaml = rule(
    implementation = _json_to_yaml_impl,
    doc = "Converts json to yaml using the yq tool",
    attrs = {
        "src": attr.label(
            doc = "Single json file",
            allow_single_file = [".json"],
            mandatory = True,
        ),
        "output": attr.string(
            doc = "Name of the output yaml file",
            mandatory = True,
        ),
        "_tool": attr.label(
            doc = "Path to yq tool",
            default = "@yq//file:file",
            executable = True,
            cfg = "exec",
        )
    }
)
