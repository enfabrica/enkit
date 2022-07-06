load("@rules_foreign_cc//foreign_cc/private:detect_root.bzl", "detect_root")
load("@rules_foreign_cc//toolchains/native_tools:tool_access.bzl", "access_tool")
load("@rules_foreign_cc//foreign_cc/private:make_script.bzl", "pkgconfig_script")
load("@rules_foreign_cc//foreign_cc/private:make_env_vars.bzl", "get_make_env_vars")
load("@rules_foreign_cc//foreign_cc/private:cc_toolchain_util.bzl", "get_flags_info", "get_tools_info")
load(
    "@rules_foreign_cc//foreign_cc/private:framework.bzl",
    "CC_EXTERNAL_RULE_ATTRIBUTES",
    "CC_EXTERNAL_RULE_FRAGMENTS",
    "cc_external_rule_impl",
    "create_attrs",
    "expand_locations",
    "expand_locations_and_make_variables",
)

def _create_meson_script(configureParameters):
    ctx = configureParameters.ctx
    attrs = configureParameters.attrs
    inputs = configureParameters.inputs

    tools = get_tools_info(ctx)
    flags = get_flags_info(ctx)
    data = ctx.attr.data + ctx.attr.build_data
    user_env = expand_locations_and_make_variables(ctx, "env", data)

    ext_build_dirs = inputs.ext_build_dirs

    script = pkgconfig_script(ext_build_dirs)

    script.append("export INSTALL_PREFIX=\"{install_prefix}\"".format(
        install_prefix=ctx.attr.name,
    ))

    setup_args = " ".join([
        expand_locations(ctx, arg, data)
        for arg in ctx.attr.setup_args
    ])

    script.append("{env_vars} {meson} setup --prefix {prefix} {setup_args} {builddir} {sourcedir}".format(
        meson = attrs.meson_path,
        setup_args = setup_args,
        env_vars = get_make_env_vars(ctx.workspace_name, tools, flags, user_env, ctx.attr.deps, inputs),
        prefix = "$$BUILD_TMPDIR$$/$$INSTALL_PREFIX$$",
        builddir = "$$BUILD_TMPDIR$$",
        sourcedir = "$$EXT_BUILD_ROOT$$/" + detect_root(ctx.attr.lib_source),
    ))

    script.append("{meson} install -C {dir}".format(
        meson = attrs.meson_path,
        dir = "$$BUILD_TMPDIR$$",
    ))

    script.append("##copy_dir_contents_to_dir## $$BUILD_TMPDIR$$/$$INSTALL_PREFIX$$ $$INSTALLDIR$$")

    return script

def _access_and_expect_label_copied(toolchain_type_, ctx):
    tool_data = access_tool(toolchain_type_, ctx, "")

    # This could be made more efficient by changing the
    # toolchain to provide the executable as a target
    cmd_file = tool_data
    for f in tool_data.target.files.to_list():
        if f.path.endswith("/" + tool_data.path):
            cmd_file = f
            break
    return struct(
        deps = [tool_data.target],
        # as the tool will be copied into tools directory
        path = "$EXT_BUILD_ROOT/{}".format(cmd_file.path),
    )

def get_meson_data(ctx):
    return _access_and_expect_label_copied(Label("@meson//:meson_toolchain_type"), ctx)

def _meson_impl(ctx):
    meson_data = get_meson_data(ctx)

    tools_deps = meson_data.deps

    attrs = create_attrs(
        ctx.attr,
        configure_name = "Meson",
        create_configure_script = _create_meson_script,
        tools_deps = tools_deps,
        meson_path = meson_data.path,
    )
    return cc_external_rule_impl(ctx, attrs)

def _attrs():
    attrs = dict(CC_EXTERNAL_RULE_ATTRIBUTES)
    attrs.update({
        "setup_args": attr.string_list(
            doc = "Arguments for the meson setup command",
            mandatory = False,
        ),
    })
    return attrs

meson = rule(
    doc = """Rule for building external library with Meson.

Here is an example on how to use this rule:

```
load("@enkit//bazel/meson:meson.bzl", "meson")

filegroup(
    name = "src",
    srcs = glob(["**"]),
)

meson(
    name = "program",
    lib_source = ":src",
    out_binaries = [
        "program",
    ],
)
```
""",
    attrs = _attrs(),
    fragments = CC_EXTERNAL_RULE_FRAGMENTS,
    output_to_genfiles = True,
    implementation = _meson_impl,
    toolchains = [
        "@bazel_tools//tools/cpp:toolchain_type",
        "@meson//:meson_toolchain_type",
        "@rules_foreign_cc//foreign_cc/private/framework:shell_toolchain",
    ],
    incompatible_use_toolchain_transition = True,
)

def meson_register_toolchains():
    native.register_toolchains("@meson//:meson_toolchain")
