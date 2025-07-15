def _cc_filter(ctx):
    src = ctx.attr.src
    lib_name = ctx.attr.name

    files = []
    linker_inputs = []

    for src_linker_input in src[CcInfo].linking_context.linker_inputs.to_list():
        libraries = []

        for src_lib in src_linker_input.libraries:
            if not src_lib.dynamic_library:
                continue
            for out_shared_lib in ctx.attr.out_shared_libs:
                lib_name = "/".join(out_shared_lib.name.split("/")[1:])
                if src_lib.dynamic_library.path.endswith(lib_name):
                    copy = ctx.actions.declare_file(out_shared_lib.name)
                    ctx.actions.symlink(output = copy, target_file = src_lib.dynamic_library)

                    files.append(copy)
                    libraries.append(src_lib)

                    break

            linker_inputs.append(
                cc_common.create_linker_input(
                    owner = src_linker_input.owner,
                    libraries = depset(direct = libraries),
                    user_link_flags = depset(direct = src_linker_input.user_link_flags),
                ),
            )

    return [
        DefaultInfo(
            files = depset(direct = files),
            runfiles = ctx.runfiles(),
        ),
        CcInfo(
            compilation_context = src[CcInfo].compilation_context,
            linking_context = cc_common.create_linking_context(
                linker_inputs = depset(direct = linker_inputs),
            ),
        ),
    ]

_cc_filter_rule = rule(
    implementation = _cc_filter,
    attrs = {
        "src": attr.label(),
        "out_shared_libs": attr.output_list(),
    },
    fragments = [
        "cpp",
    ],
    toolchains = [
        "@bazel_tools//tools/cpp:toolchain_type",
    ],
)

def cc_filter(name, src, out_shared_libs, **kwargs):
    """
    This rule filters the outputs of the rule `src` to the ones defined in `out_shared_libs`.

    For example:
    ```
    cmake(
        name = "project",
        out_shared_libs = [
            "libproject1.so",
            "libproject2.so",
            "libproject3.so",
        ],
    )
    cc_filter(
        name = "project-filtered",
        src = ":project",
        out_shared_libs = [
            "libproject1.so",
            "libproject2.so",
        ],
    )
    cc_binary(name = "binary1", srcs = ["binary.c"], deps = [":project"])
    cc_binary(name = "binary2", srcs = ["binary.c"], deps = [":project-filtered"])
    ```

    The first binary (`binary1`) will link against all 3 libraries. The other binary (`binary2`)
    will link against 2 of them.

    The rule will only filter dynamic libraries. In a future revision, we will add support for
    static libraries as well.
    """

    out_shared_libs = ["/".join([name, out_shared_lib]) for out_shared_lib in out_shared_libs]

    return _cc_filter_rule(
        name = name,
        src = src,
        out_shared_libs = out_shared_libs,
        **kwargs
    )
