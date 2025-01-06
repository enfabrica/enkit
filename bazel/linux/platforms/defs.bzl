load("//bazel/linux:providers.bzl", "KernelBundleInfo")

# List of constraints for aarch64 kernel module building.  This is
# used during toolchain resolution.
kernel_aarch64_constraints = [
    "@platforms//cpu:aarch64",
    "@platforms//os:linux",
    Label("//bazel/linux/platforms:gcc_13"),
]

def _kernel_aarch64_transition_impl(settings, attr):
    return {
        "//command_line_option:platforms": "//bazel/linux/platforms:kernel_aarch64",
    }

# A transition to the kernel_aarch64 platform
kernel_aarch64_target = transition(
    implementation = _kernel_aarch64_transition_impl,
    inputs = [],
    outputs = [
        "//command_line_option:platforms",
    ],
)

def _kernel_module_passthrough(ctx):
    # Unfortuanetly we cannot iterate through the providers in a Target, so add support for
    # the providers we know about.
    bundle_info = [target[KernelBundleInfo] for target in ctx.attr.target if KernelBundleInfo in target]

    return [DefaultInfo(
        files = depset(
            [],
            transitive = [target[DefaultInfo].files for target in ctx.attr.target],
        ),
    )] + bundle_info

kernel_aarch64_transition = rule(
    implementation = _kernel_module_passthrough,
    attrs = {
        "target": attr.label(cfg = kernel_aarch64_target),
        "_allowlist_function_transition": attr.label(
            doc = "bazel magic that allows transitions to work",
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
)
