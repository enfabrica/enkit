"""A fork of codegen.bzl.

This is phoney-baloney documentation for testing the bzldoc flow.

Here is *some* _markdown_ embedded in a docstring.
"""

load("//bazel:cc.bzl", "en_cc_library")
load("//bazel:utils.bzl", "diff_test", "uniquify")
load(
    "//bazel/eda:verilog.bzl",
    "verilog_library",
)

def _elab_impl(ctx):
    args = ctx.actions.args()
    args.add("--alsologtostderr")

    # Load deps first:
    for f in ctx.files.data_deps:
        args.add("--load", f)

    for d in ctx.files.deps:
        args.add("--load", d.path)

    # Load primary data files last:
    for f in ctx.files.data:
        args.add("--load", f)
    args.add("--output", ctx.outputs.out.path)

    ctx.actions.run(
        inputs = ctx.files.data + ctx.files.data_deps,
        outputs = [ctx.outputs.out],
        executable = ctx.executable.elab_tool,
        arguments = [args],
        progress_message = "Generating %s" % ctx.outputs.out.short_path,
    )

elab = rule(
    doc = """
      Runs elab to elaborate data files.
    """,
    implementation = _elab_impl,
    output_to_genfiles = True,  # so that header files can be found.
    attrs = {
        "data": attr.label_list(
            allow_files = [".json", ".yaml"],
            doc = "A list of data files to load.",
        ),
        "deps": attr.label_list(
            doc = "A list of codegen_design target dependencies.",
        ),
        "data_deps": attr.label_list(
            allow_files = [".json", ".yaml"],
            doc = "A list of data files dependencies to load.",
        ),
        "out": attr.output(
            mandatory = True,
            doc = "The single artifact file to generate.",
        ),
        "overrides": attr.string_list(doc = "A pair of key=value pairs to override context data."),
        "elab_tool": attr.label(
            executable = True,
            cfg = "exec",
            allow_files = True,
            default = Label("//tools/codegen:elab"),
            doc = "The path to the elab tool itself.",
        ),
    },
)

def _codegen_impl(ctx):
    args = ctx.actions.args()
    for f in ctx.files.schema:
        args.add("--schema", f)
    for f in ctx.files.data:
        args.add("--load", f)
    for f in ctx.files.srcs:
        args.add(f)
    if ctx.attr.multigen_mode:
        args.add("--multigen_mode")
    for f in ctx.outputs.outs:
        args.add("--output", f.path)

    ctx.actions.run(
        inputs = ctx.files.data + ctx.files.srcs + ctx.files.schema,
        outputs = ctx.outputs.outs,
        executable = ctx.executable.codegen_tool,
        arguments = [args],
        progress_message = "Generating %s" % ",".join([repr(x.short_path) for x in ctx.outputs.outs]),
    )

codegen = rule(
    doc = """
      Runs codegen to combine templates and data files to an artifact.

      TODO(jonathan): generalize this to generate multiple artifacts.
    """,
    implementation = _codegen_impl,
    output_to_genfiles = True,  # so that header files can be found.
    attrs = {
        "data": attr.label_list(
            allow_files = [".json", ".yaml"],
            doc = "An ordered list of data files to load.",
        ),
        "outs": attr.output_list(
            allow_empty = False,
            doc = "Artifacts to generate.",
        ),
        "srcs": attr.label_list(
            allow_files = [".jinja2", ".jinja", ".template"],
            doc = "A list of jinja2 template files to import.",
        ),
        "schema": attr.label(
            allow_files = [".schema", "schema.yaml"],
            doc = "A jsonschema file to check the imported data against.",
        ),
        "overrides": attr.string_list(doc = "A pair of key=value pairs to override context data."),
        "template_name": attr.string(doc = "The specific jinja2 template to render (optional)."),
        "multigen_mode": attr.bool(doc = "Enable multigen mode."),
        "codegen_tool": attr.label(
            executable = True,
            cfg = "exec",
            allow_files = True,
            default = Label("//tools/codegen:codegen"),
            doc = "The path to the codegen tool itself.",
        ),
    },
)

def codegen_test(name, expected = None, **codegen_args):
    codegen(
        name = name + "-actual-gen",
        outs = [name + ".actual"],
        **codegen_args
    )
    if not expected:
        expected = name + ".expected"
    diff_test(
        name = name,
        actual = name + "-actual-gen",
        expected = expected,
    )

# find a better home for this:
def en_dpi_library(name, **kwargs):
    dpi_kwargs = {}
    dpi_kwargs.update(**kwargs)
    dpi_kwargs["copts"] = uniquify(dpi_kwargs.get("copts", []) + [
        "-fPIC",
    ])
    dpi_kwargs["deps"] = uniquify(dpi_kwargs.get("deps", []) + [
        "@Cadence//:headers",
    ])
    en_cc_library(
        name = name,
        **dpi_kwargs
    )

def codegen_design(name, config, deps = []):
    """Generates all supported code from a design specification.

    The provided data file must follow the "design" schema for specifying
    design elements.

    The following rules are generated by this macro:

    * `<name>-config_elab`: produces `<name>-elab.yaml`, the elaborated data set.
    * `<name>-sv`: produces `<name>_pkg.sv`, the generated SystemVerilog package file.
    * `<name>-model-h-gen`: produces `<name>.model.h`, the performance model C++ header.
    * `<name>-model-cc-gen`: produces `<name>.model.cc`, the performance model C++ implementation.
    * `<name>-model`: the library compiling the previous two targets.
    * `<name>-sysc-gen`: produces `<name>.sysc.h`, the SystemC model header (deprecated).
    * `<name>-sysc`: the library compiling the previous target (deprecated).

    TODO(jonathan): This scheme isn't perfect.  The first problem is that we need a
    separate codegen_design rule for each yaml file.  The second problem is that
    this macro assumes all files are in the same directory, and it's not clear that
    the #includes will be generated correctly for dependencies in other directories.
    Fixing this will require a little bazel wizardry, so I'm postponing that for a
    future PR.

    Args:
      name: the name of the macro.  Must be the same as the `name` attribute in
          the config file.
      config: the YAML or JSON data file.  Must conform to the "design" schema
          defined in //tools/codegen/schema:design_schema.yaml.
      deps: the name of other codegen_design dependencies.  Each dependency
          must be declared with its own codegen_design rule.
    """
    if config != name + ".yaml":
        print("Warning: config file %r should be named %r." % (config, name + ".yaml"))
    elaborated_config = name + ".elab.yaml"
    data_deps = ["%s.yaml" % x for x in deps]
    elab(
        name = name + "-config_elab",
        data = [config],
        out = elaborated_config,
        data_deps = data_deps,
        visibility = ["//visibility:public"],
    )
    codegen(
        name = name + "-model-h-gen",
        outs = [name + ".model.h"],
        srcs = ["//tools/codegen/templates:model_structs.h.template"],
        data = [elaborated_config],
    )
    codegen(
        name = name + "-model-cc-gen",
        outs = [name + ".model.cc"],
        srcs = ["//tools/codegen/templates:model_structs.cc.template"],
        data = [elaborated_config],
    )
    en_cc_library(
        name = name + "-model",
        srcs = [name + ".model.cc"],
        hdrs = [name + ".model.h"],
        deps = [
            "@com_github_google_glog//:glog",
            "//model/perf/common:bitslice",
        ] + ["%s-model" % x for x in deps],
        visibility = ["//visibility:public"],
    )
    codegen(
        name = name + "-sysc-gen",
        outs = [name + ".sysc.h"],
        srcs = ["//tools/codegen/templates:sysc_structs.h.template"],
        data = [elaborated_config],
    )
    en_cc_library(
        name = name + "-sysc",
        srcs = [],
        hdrs = [name + ".sysc.h"],
        deps = ["@systemc//:libsystemc"] + ["%s-sysc" % x for x in deps],
        visibility = ["//visibility:public"],
    )
    codegen(
        name = name + "-sv-gen",
        outs = [name + "_pkg.sv"],
        srcs = ["//tools/codegen/templates:sv_structs.h.template"],
        data = [elaborated_config],
        visibility = ["//visibility:public"],
    )
    verilog_library(
        name = name + "-sv",
        srcs = [name + "_pkg.sv"],
        deps = ["%s-sv" % x for x in deps],
        visibility = ["//visibility:public"],
    )
    codegen(
        name = name + "-md-gen",
        outs = [name + ".md"],
        srcs = ["//tools/codegen/templates:documentation.md.template"],
        data = [elaborated_config],
        visibility = ["//visibility:public"],
    )
    # TODO(jonathan): re-enable when fixed.
    # codegen(
    #     name = name + "-dpi-gen",
    #     outs = [name + ".dpi.cc"],
    #     srcs = ["//tools/codegen/templates:dpi_structs.h.template"],
    #     data = [elaborated_config],
    # )
    # en_dpi_library(
    #     name = name + "-dpi",
    #     srcs = [name + ".dpi.cc"],
    #     visibility = ["//visibility:public"],
    # )
