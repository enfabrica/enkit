"""Cloud Run rules"""

_ACCESS_MODES = [
    "authenticated",
    "public",
]

def _cloud_run_deploy_impl(ctx):
    access_flag = "--allow-unauthenticated" if ctx.attr.access == "public" else "--no-allow-unauthenticated"
    http2_flag = "--use-http2" if not ctx.attr.http2_downgrade else "--no-use-http2"
    port_flag = "--port={}".format(ctx.attr.port)
    args = " \\\n  ".join(['--args="{}"'.format(a) for a in ctx.attr.container_args])

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
  {http2_mode} \\
  {port} \\
  {args}
""".format(
            project = ctx.attr.project,
            service_name = ctx.attr.service,
            image = image,
            region = ctx.attr.region,
            access = access_flag,
            http2_mode = http2_flag,
            port = port_flag,
            args = args,
        ),
        is_executable = True,
    )
    return DefaultInfo(executable = run_script)

cloud_run_deploy = rule(
    implementation = _cloud_run_deploy_impl,
    attrs = {
        "project": attr.string(
            mandatory = True,
            doc = "gcloud project to which to deploy Cloud Run service",
        ),
        "service": attr.string(
            mandatory = True,
            doc = "Name of service to deploy",
        ),
        "image": attr.string(
            mandatory = True,
            doc = "Repository and image path to Docker image to deploy",
        ),
        "image_version": attr.string(
            mandatory = True,
            doc = "Image tag or hash to deploy. If a hash, must start with `sha256:`",
        ),
        "region": attr.string(
            mandatory = True,
            doc = "Region to which to deploy Cloud Run service",
        ),
        "access": attr.string(
            default = "authenticated",
            values = _ACCESS_MODES,
            doc = "Defines who can access the service. If set to `public`, anyone can issue gRPC/HTTP requests.",
        ),
        "http2_downgrade": attr.bool(
            default = False,
            doc = "If true, service will only support HTTP2. If false, gcloud will downgrade HTTP2 to HTTP1.1 in the load balancer before the service.",
        ),
        "container_args": attr.string_list(
            doc = "List of args to pass to the container",
        ),
        "port": attr.int(
            default = 8080,
            doc = "Port on which service listens for requests. If the container listens on $PORT, this can be left as the default. If the container listens on a hard-coded port, this should be set.",
        ),
    },
    executable = True,
)
