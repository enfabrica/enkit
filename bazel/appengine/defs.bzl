"""AppEngine Bazel rules

Forked from: https://github.com/ccontavalli/bazel-rules/blob/master/appengine/defs.bzl
"""

load("@rules_go//go:def.bzl", "GoPath", "go_path")

def _go_appengine_deploy_path_impl(ctx):
    config = ctx.file.config
    gp = ctx.attr.path[GoPath]

    args = [
        "-config=" + config.path,
        "-entry=" + ctx.attr.entry,
        "-gcloud=" + ctx.attr.gcloud,
    ]
    inputs = [ctx.executable._deploy, gp.gopath_file, config]
    if ctx.attr.gomod:
        args.append("-gomod=" + ctx.file.gomod.path)
        inputs.append(ctx.file.gomod)
    if ctx.attr.gosum:
        args.append("-gosum=" + ctx.file.gosum.path)
        inputs.append(ctx.file.gosum)
    extra = "--project={project}".format(project = ctx.attr.project)
    if ctx.attr.extra:
        extra = extra + " " + " ".join([s.replace("'", "'\\''") for s in ctx.attr.extra])
    args.append("-extra='" + extra + "'")

    gcloud = ctx.actions.declare_file("gcloud.sh")
    ctx.actions.expand_template(
        template = ctx.file._gcloud,
        output = gcloud,
        substitutions = {
            "{ropath}": gp.gopath,
            "{binary}": ctx.executable._deploy.short_path,
            "{args}": " ".join(args),
        },
        is_executable = True,
    )

    return [DefaultInfo(executable = gcloud, runfiles = ctx.runfiles(inputs))]

go_appengine_deploy_path = rule(
    implementation = _go_appengine_deploy_path_impl,
    executable = True,
    attrs = {
        "path": attr.label(
            mandatory = True,
            providers = [GoPath],
            doc = "A go_path() target defining a set of go files",
        ),
        "gomod": attr.label(
            allow_single_file = True,
            doc = "A go.mod file, to be included in the push to GAE",
        ),
        "gosum": attr.label(
            allow_single_file = True,
            doc = "A go.sum file, to be included in the push to GAE",
        ),
        "entry": attr.string(
            mandatory = True,
            doc = "The entry point where your application is located (eg, github.com/ccontavalli/myapp)",
        ),
        "project": attr.string(
            mandatory = True,
            doc = "GCP project to push to",
        ),
        "gcloud": attr.string(
            mandatory = False,
            default = "/usr/bin/gcloud",
            doc = "Path to the system installed gcloud binary",
        ),
        "config": attr.label(
            allow_single_file = True,
            mandatory = True,
            doc = "The app.yaml configuration file to use for gcloud app deploy",
        ),
        "extra": attr.string_list(
            allow_empty = True,
            doc = "Extra flags to pass to gcloud",
        ),
        "_deploy": attr.label(
            default = Label("//bazel/appengine/deploy"),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
        "_gcloud": attr.label(
            default = Label("//bazel/appengine:gcloud.sh"),
            allow_single_file = True,
        ),
    },
)

def go_appengine_deploy(name, entry, deps, config, **kwargs):
    # Packs all the dependencies in some place.
    go_path(name = name + "-dir", mode = "copy", deps = deps)
    go_appengine_deploy_path(name = name, path = ":" + name + "-dir", entry = entry, config = config, **kwargs)
