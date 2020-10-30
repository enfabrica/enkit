def validate_format(tool, output, inputfiles, strip = "", format = "", execution_requirements = None):
    """Verifies that the input file is of the valid format specified.

    Args:
      output: a file where all the validated files will be copied into. An output file is required by bazel.
      inputfile: a list of File objects, to verify.
      strip: an optional extension to strip from the file to guess the format.
      format: string, the format of the file. Can be 'json', 'yaml' or any format supported by
         the github.com/enfabrica/enkit/lib/config/unmarshal library.
         If left empty, the format is determined by the extension of the file.

    Returns:
      A dict that can be simply passed as **kwarg to ctx.actions.run(), or mangled before it is
      passed over.

    Example:
      my_rule_implementation(ctx):
        ...
        expander = validate_format(ctx.file._tool, validate = ctx.file.config, output = output, subs = subs)
        ctx.actions.run(**expander)

    In the rule definition:
      ...
      attrs = {
        "_tool": validate_tool,
      }
    """
    args = ["-output", output.path]
    if format:
        args.extend(["-format", format])

    if strip:
        args.extend(["-strip", strip])

    inputs = []
    for input in inputfiles:
        args.append(input.path)

    return dict(
        executable = tool,
        arguments = args,
        outputs = [output],
        inputs = inputfiles,
        execution_requirements = execution_requirements,
    )

validate_tool = attr.label(
    default = Label("//bazel/utils/validate"),
    cfg = "host",
    executable = True,
    allow_single_file = True,
)
