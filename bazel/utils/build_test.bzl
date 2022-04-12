def _test_wrapper(ctx):
    """Turns a build target into a test target.

    The test_suite() rule only accepts bazel test targets, but
    sometimes it is useful to include a build target as a test
    for infrastructure changes.
    """
    runfiles = ctx.runfiles(files = ctx.attr.dep.files.to_list())
    script = ctx.actions.declare_file("{}.sh".format(ctx.attr.name))

    # By bazel convention, the test rule must return an executable script,
    # even if the script doesn't run anything.
    ctx.actions.write(script, "")
    return [DefaultInfo(runfiles = runfiles, executable = script)]

build_test = rule(
    implementation = _test_wrapper,
    test = True,
    attrs = {
        "dep": attr.label(
            doc = "Build target to be converted into a test target",
            mandatory = True,
        ),
    },
)
