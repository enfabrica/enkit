def template_expand(tool, template, output, subs, executable = False, execution_requirements = None):
    """Powerful template expansion rule.

    template_expand uses the powerful golang text/template engine to expand an input template
    into an output file. The subs dict provide keys and values, where the values can either be
    plain strings, or File objects that end up read from disk.

    The full documentation of text/template syntax can be found here:
        https://golang.org/pkg/text/template/

    Args:
      template: a File object, the template to expand. The template can use any syntax supported
         by the golang text/template library.
      output: a File object, the file to create with the expanded template.
      subs: a dict, {"key": value}, with key being a string, and value being either a string, or
         a file object. If the value is a file, the corresponding file will be read and expanded
         every time key is used.

         If the key is set to "myKey", you can reference it in the template by either using
         {{.myKey}}, or by using {{.Get "myKey"}}. Using ".Get" allows to access keys that use
         invalid go syntax (for example, "my-key:next"), and causes the substitution to fail if
         the key is not present.

    Returns:
      A dict that can be simply passed as **kwarg to ctx.actions.run(), or mangled before it is
      passed over.

    Example:
      my_rule_implementation(ctx):
        ...
        expander = template_expand(ctx.file._tool, template = ctx.file.config, output = output, subs = subs)
        ctx.actions.run(**expander)

    In the rule definition:
      ...
      attrs = {
        "_tool": template_tool,
      }
    """
    args = [
        "-template",
        template.path,
        "-output",
        output.path,
    ]
    if executable:
        args.append("-executable")

    inputs = [template]
    for key, value in subs.items():
        args.extend(["-key", key])
        if type(value) == "string":
            args.extend(["-value", value])
        else:
            inputs.append(value)
            args.extend(["-valuefile", value.path])

    return dict(
        executable = tool,
        arguments = args,
        inputs = inputs,
        outputs = [output],
        execution_requirements = execution_requirements,
    )

# Use template_tool in your rule definition, and pass the corresponding attribute
# to template_execute as the first parameter.
#
# For example:
#
#    example_rule = rule(
#        implementation = _example_rule_impl,
#        attrs = {
#            "output": attr.string(...),
#            [...]
#            "_expander": template_tool,
#        }
#    )
#
#    def _example_rule_impl(ctx):
#      [...]
#      expander = template_expand(ctx.attr._expander, ...)
#      ctx.actions.run(**expander)
#
template_tool = attr.label(
    default = Label("//bazel/utils/template"),
    cfg = "host",
    executable = True,
    allow_single_file = True,
)

def expand_template_binary_impl(ctx):
    output = ctx.actions.declare_file(ctx.attr.output)
    ctx.actions.expand_template(
        is_executable = ctx.attr.executable,
        template = ctx.file.template,
        output = output,
        substitutions = ctx.attr.substitutions,
    )
    return [
        DefaultInfo(
            files = depset([output]),
            executable = output if ctx.attr.executable else None,
        )
    ]

expand_template_binary = rule(
    implementation = expand_template_binary_impl,
    attrs = {
        "executable": attr.bool(
            doc = "Make the generated file executable",
            default = False,
        ),
        "template": attr.label(
            doc = "Template file interpretted by the expand_template bazel action",
            allow_single_file = True,
            mandatory = True,
        ),
        "substitutions": attr.string_dict(
            doc = "Keys are '{{key}}' and values are plain strings",
            mandatory = True,
        ),
        "output": attr.string(
            doc = "Name of the output file",
            mandatory = True,
        ),
    }
)

