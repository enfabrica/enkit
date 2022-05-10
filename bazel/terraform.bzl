load("@bazel_skylib//lib:paths.bzl", "paths")

TerraformConfigInfo = provider(
    doc = "Propagates Terraform configs through rules",
    fields = {
        "configs": "Depset of configs to apply",
        "data": "Depset of data dependencies for configs",
    },
)

def _terraform_library_impl(ctx):
    return [
        DefaultInfo(
            files = depset(direct = ctx.files.srcs + ctx.files.data),
        ),
        TerraformConfigInfo(
            configs = depset(direct = ctx.files.srcs),
            data = depset(direct = ctx.files.data),
        ),
    ]

terraform_library = rule(
    implementation = _terraform_library_impl,
    doc = "Wraps Terraform config files so they can be propagated to rules expecting Terraform",
    attrs = {
        "srcs": attr.label_list(
            mandatory = True,
            doc = "Terraform files to wrap",
            allow_files = [".tf"],
        ),
        "data": attr.label_list(
            doc = "Extra files referenced by terraform configs (templates, file()...)",
            allow_files = True,
        )
    },
)

def _terraform_apply_impl(ctx):
    outputs = []

    stage_dir = "{}_stage".format(ctx.attr.name)

    # Symlink lock file into stage dir
    lock_file = ctx.actions.declare_file("{}/.terraform.lock.hcl".format(stage_dir))
    ctx.actions.symlink(output = lock_file, target_file = ctx.file.terraform_lock)
    outputs.append(lock_file)

    # Symlink all the configs into stage dir
    for target in ctx.attr.configs:
        tfc = target[TerraformConfigInfo]
        for f in tfc.configs.to_list():
            symlinked_f = ctx.actions.declare_file(f.basename, sibling = lock_file)
            ctx.actions.symlink(output = symlinked_f, target_file = f)
            outputs.append(symlinked_f)

        if not hasattr(tfc, "data"):
            continue

        for f in tfc.data.to_list():
            symlinked_f = ctx.actions.declare_file(f.short_path, sibling = lock_file)
            ctx.actions.symlink(output = symlinked_f, target_file = f)
            outputs.append(symlinked_f)

    # Generate script that can run terraform commands in stage dir
    script_file = ctx.actions.declare_file("{}_run.sh".format(ctx.attr.name))
    script = """#!/bin/bash
pwd

terraform -chdir={stage_dir} init
terraform -chdir={stage_dir} "$@"
""".format(
        stage_dir = paths.dirname(lock_file.short_path),
    )
    ctx.actions.write(script_file, script, is_executable = True)
    outputs.append(script_file)

    return [
        DefaultInfo(
            files = depset(direct = outputs),
            runfiles = ctx.runfiles(files = outputs),
            executable = script_file,
        ),
    ]

terraform_apply = rule(
    implementation = _terraform_apply_impl,
    doc = "Generates a directory with all configs symlinked in, and a script to run Terraform operations on said directory",
    attrs = {
        "configs": attr.label_list(
            mandatory = True,
            doc = "Targets providing Terraform configuraion",
            providers = [TerraformConfigInfo],
        ),
        "terraform_lock": attr.label(
            mandatory = True,
            doc = "Terraform lockfile that describes the versions of plugins to use",
            allow_single_file = [".lock.hcl"],
        ),
    },
    executable = True,
)
