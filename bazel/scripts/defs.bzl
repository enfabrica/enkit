def _my_rule_impl(ctx):
    #Make flatmap helper?
    all_files = []
#    print(ctx.attr.go_library)
    #for lib in ctx.attr.go_library:
    lib = ctx.attr.go_library[0]
    print(lib)
    print(lib.files.to_list())
#    print(lib.transitive_sources.to_list())
#    TransitiveDataInfo = provider("deps")
#    print(lib[TransitiveDataInfo])
#    print(lib[TransitiveDataInfo])
    a = lib.files.to_list()
    print(a[0].is_directory)
#    for go_file in lib.files:
#        print("here")
#        print("here")
#        print(go_file.to_list())

    all_deps = []
    all_deps.append(a[0])
#    all_deps.append()
    print("running this script")
    print(ctx.attr._lint_script.files.to_list()[0].basename)
    print(ctx.executable._compiler)
    print([lib.files] + [ctx.executable._compiler])
    ctx.actions.run(
        inputs = lib.files,
        outputs = [ctx.outputs.output],
        arguments = [],
        progress_message = "Running linter into",
        executable = ctx.executable._lint_script,
        tools = [ctx.executable._compiler],
        use_default_shell_env = True,
        env = {
            "enkit": ctx.executable._compiler.path
        }
    )
    print("ran all the stuff")
#    return DefaultInfo(files = depset(all_deps))



go_lint = rule(
    _my_rule_impl,
    attrs = {
        "go_library": attr.label_list(),
        "output": attr.output(mandatory = True),
        "_lint_script": attr.label(
            default = Label("//bazel/scripts:lint.sh"),
            allow_files = True,
            executable = True,
            cfg = "exec"
        ),
        "_compiler": attr.label(
            default = Label("//enkit:enkit-darwin-amd64"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "_go": attr.label(
            default = Label("//enkit:enkit-darwin-amd64"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    }
)

#def _whl_deps_filegroup_impl(ctx):
#    input_wheels = ctx.attr.src[_PyZProvider].transitive_wheels
#    output_wheels = []
#    for wheel in input_wheels:
#        file_name = wheel.basename
#        output_wheel = ctx.actions.declare_file(file_name)
#        ctx.actions.run(
#            outputs=[output_wheel],
#            inputs=[wheel],
#            arguments=[wheel.path, output_wheel.path],
#            executable="cp",
#            mnemonic="CopyWheel")
#        output_wheels.append(output_wheel)
#
#    return [DefaultInfo(files=depset(output_wheels))]
#
#whl_deps_filegroup = rule(
#    _whl_deps_filegroup_impl,
#    attrs = {
#        "src": attr.label(),
#    },
#)
#
#
#genrule(
#    name = "go_lint_1",
#    srcs = [
#        "//astore/client:go_default_library",  # a filegroup with multiple files in it ==> $(locations)
#    ],
#    outs = ["concatenated.txt"],
#    cmd = "golangci-lint --path-prefix $(locations //astore/client:go_default_library) > $@",
#)

