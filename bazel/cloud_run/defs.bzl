"""Cloud Run rules"""

_ACCESS_MODES = [
    "authenticated",
    "public",
]

def _cloud_run_deploy_impl(ctx):
    if ctx.attr.access not in _ACCESS_MODES:
        fail("attribute `access` must be one of: {}".format(_ACCESS_MODES))
    access_flag = "--allow-unauthenticated" if ctx.attr.access == "public" else "--no-allow-unauthenticated"

    http2_flag = "--use-http2" if not ctx.attr.http2_downgrade else "--no-use-http2"

    if ctx.attr.image_version.startswith("sha256:"):
        image = "{}@{}".format(ctx.attr.image, ctx.attr.image_version)
    else:
        image = "{}:{}".format(ctx.attr.image, ctx.attr.image_version)

    run_script = ctx.actions.declare_file("{}.sh".format(ctx.attr.name))
    ctx.actions.write(
        run_script,
        """#!/bin/bash
gcloud \\
  --project={project} \\
  run \\
  deploy \\
  {service_name} \\
  --image {image} \\
  --region {region} \\
  {access} \\
  {http2_mode}
""".format(
            project = ctx.attr.project,
            service_name = ctx.attr.service,
            image = image,
            region = ctx.attr.region,
            access = access_flag,
            http2_mode = http2_flag,
        ),
        is_executable = True,
    )
    print("cloud_run_deploy!")
    return DefaultInfo(executable = run_script)

cloud_run_deploy = rule(
    implementation = _cloud_run_deploy_impl,
    attrs = {
        "project": attr.string(
            mandatory = True,
        ),
        "service": attr.string(
            mandatory = True,
        ),
        "image": attr.string(
            mandatory = True,
        ),
        "image_version": attr.string(
            mandatory = True,
        ),
        "region": attr.string(
            mandatory = True,
        ),
        "access": attr.string(
            default = "authenticated",
        ),
        "http2_downgrade": attr.bool(
            default = False,
        ),
    },
    executable = True,
)
