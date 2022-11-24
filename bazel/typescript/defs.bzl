# The state of typescript support and gRPC is a mess as of Nov/2022.
#
# Tl;Dr: most rulesets generating ts or js from proto file depend on
# rules_nodejs, which is officially deprecated.
#
# rules_js, significantly better, is not supported out of the box by those 3rd
# party rulesets.
#
# Turns out that it's simple to integrate rules_js with rules_proto_grpc
# with little glue code, thanks to:
#   https://github.com/aspect-build/rules_js/issues/397
#
# ... but most of the required PRs are not merged upstream yet.
# Most of the code here is cut and paste from that bug, with local adaptations.
#
# TODO: this should match what will soon be upstream, remove this integration
#       once upstream rules are more mature.
#
# More details at:
#    https://bazel-contrib.github.io/SIG-rules-authors/proto-grpc.html#the-ruleset-options
#
load(
    "@rules_proto_grpc//:defs.bzl",
    "ProtoPluginInfo",
    "proto_compile_attrs",
    "proto_compile_impl",
)

def _ts_proto_compile_impl(ctx):
    """
    Implementation function for ts_proto_compile.

    Args:
        ctx: The Bazel rule execution context object.

    Returns:
        Providers:
            - ProtoCompileInfo
            - DefaultInfo

    """
    # base_env = {
    #     # Make up for https://github.com/bazelbuild/bazel/issues/15470.
    #     "BAZEL_BINDIR": ctx.bin_dir.path,
    # }
    # return proto_compile_impl(ctx, base_env = base_env)
    return proto_compile_impl(ctx)

ts_proto_compile = rule(
    implementation = _ts_proto_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        _plugins = attr.label_list(
            providers = [ProtoPluginInfo],
            default = [
                Label("//bazel/typescript:ts_proto_compile"),
            ],
            doc = "List of protoc plugins to apply",
        ),
    ),
    toolchains = [
        str(Label("@rules_proto_grpc//protobuf:toolchain_type")),
    ],
)
