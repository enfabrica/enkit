load("//bazel/utils:files.bzl", "write_to_file")
load("@enkit//bazel/utils:merge_kwargs.bzl", "merge_kwargs")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_push", "oci_tarball")
load("@enkit_pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

_IMAGE_BUILDER_SH = """\
#!/bin/bash
{tool} \\
    --image_definition_json={image_def} \\
    --labels={labels} \\
    --dev_repo={dev_repo} \\
    --staging_repo={staging_repo} \\
    --prod_repo={prod_repo} \\
    --v=1 \\
    $@
"""

def nonhermetic_image_builder_impl(ctx):
    script = ctx.actions.declare_file("{}_run.sh".format(ctx.attr.name))
    ctx.actions.write(script, _IMAGE_BUILDER_SH.format(
        tool = ctx.executable._tool.short_path,
        image_def = ctx.file.src.short_path,
        labels = ctx.file.labels.short_path,
        dev_repo = ctx.file.dev_repo.short_path,
        staging_repo = ctx.file.staging_repo.short_path,
        prod_repo = ctx.file.prod_repo.short_path,
    ))
    direct_files = ctx.files.src + ctx.files.labels + ctx.files.dev_repo + ctx.files.staging_repo + ctx.files.prod_repo
    runfiles = ctx.runfiles(
        files = ctx.attr._tool[DefaultInfo].files.to_list() + direct_files,
    )
    return [
        DefaultInfo(
            runfiles = runfiles,
            executable = script,
        ),
    ]

_nonhermetic_image_builder = rule(
    implementation = nonhermetic_image_builder_impl,
    attrs = {
        "src": attr.label(
            allow_single_file = [".json"],
        ),
        "labels": attr.label(
            doc = "Docker labels in the form of key-value pairs delimited by '='.",
            allow_single_file = [".txt"],
        ),
        "dev_repo": attr.label(
            doc = "Path to a file that contains the destination dev repo path.",
            mandatory = True,
            allow_single_file = [".txt"],
        ),
        "staging_repo": attr.label(
            doc = "Path to a file that contains the destination staging repo path.",
            mandatory = True,
            allow_single_file = [".txt"],
        ),
        "prod_repo": attr.label(
            doc = "Path to a file that contains the destination prod repo path.",
            mandatory = True,
            allow_single_file = [".txt"],
        ),
        "_tool": attr.label(
            default = "//bazel/utils/container/muk:muk",
            executable = True,
            cfg = "exec",
        ),
    },
    executable = True,
)

def nonhermetic_image_builder(*args, **kwargs):
    name = kwargs.get("name")
    tags = kwargs.get("tags", [])
    for repo in ["dev", "staging", "prod"]:
        container_repo(
            name = "{}_{}".format(name, repo),
            image_path = kwargs.get("image_path"),
            repo_type = repo,
            namespace = kwargs.get("namespace"),
            region = kwargs.get("region", GCP_REGION),
            project = kwargs.get("project", GCP_PROJECT),
            tags = tags,
        )
    output = "{}_labels.txt".format(name)
    user_labels = "{}_user_labels".format(name)
    write_to_file(
        name = user_labels,
        output = "{}_user_labels.txt".format(name),
        content = "".join(["{}={}\n".format(k, v) for k, v in kwargs.get("labels", {}).items()]),
        tags = tags,
    )
    native.genrule(
        name = "{}_labels".format(name),
        outs = [output],
        srcs = [":{}".format(user_labels)],
        # The full repo path needs to be specified so that when this target is dynamically
        # created in the internal repo, bazel refers to the container_stamper target in enkit
        # instead of trying to find it under @enfabrica//bazel/utils/container
        tools = ["@enkit//bazel/utils/container:container_stamper"],
        cmd = "$(location @enkit//bazel/utils/container:container_stamper) --user_labels $(location :{}) --output $(location {})".format(user_labels, output),
        tags = tags,
    )
    kwargs.pop("image_path")
    kwargs.pop("namespace")
    kwargs["labels"] = "{}_labels".format(name)
    kwargs["dev_repo"] = "{}_dev".format(name)
    kwargs["staging_repo"] = "{}_staging".format(name)
    kwargs["prod_repo"] = "{}_prod".format(name)
    _nonhermetic_image_builder(*args, **kwargs)

def container_image(*args, **kwargs):
    name = kwargs.get("name")
    output = "{}_labels.txt".format(name)
    tags = kwargs.get("tags", [])

    # Always include user-defined container labels in addition to build metadata from bazel --stamp
    # https://bazel.build/docs/user-manual#workspace-status
    # Container labels for oci_image can be a dictionary or file of key-value pairs with '=' as delimiters
    # https://github.com/bazel-contrib/rules_oci/blob/main/docs/image.md
    user_labels = "{}_user_labels".format(name)
    write_to_file(
        name = user_labels,
        output = "{}_user_labels.txt".format(name),
        content = "".join(["{}={}\n".format(k, v) for k, v in kwargs.get("labels", {}).items()]),
        tags = tags,
    )
    native.genrule(
        name = "{}_labels".format(name),
        outs = [output],
        srcs = [":{}".format(user_labels)],
        tools = ["@enkit//bazel/utils/container:container_stamper"],
        cmd = "$(location @enkit//bazel/utils/container:container_stamper) --user_labels $(location :{}) --output $(location {})".format(user_labels, output),
        tags = tags,
    )
    kwargs["labels"] = ":{}_labels".format(name)
    oci_image(*args, **kwargs)

def container_tarball(*args, **kwargs):
    oci_tarball(*args, **kwargs)

def container_push(*args, **kwargs):
    target_basename = kwargs.get("name")
    namespace = kwargs.get("namespace")
    region = kwargs.get("region", GCP_REGION)
    project = kwargs.get("project", GCP_PROJECT)
    image_path = kwargs.get("image_path")
    tags = kwargs.get("tags", [])
    for repo in ["dev", "staging"]:
        container_repo(
            name = "{}_{}".format(target_basename, repo),
            image_path = image_path,
            repo_type = repo,
            namespace = namespace,
            region = region,
            project = project,
            tags = tags,
        )
        oci_push(
            name = "{}_{}_oci_push".format(target_basename, repo),
            image = kwargs.get("image"),
            remote_tags = kwargs.get("remote_tags"),
            repository_file = ":{}_{}".format(target_basename, repo),
            tags = tags,
        )
    local_image_path = "{}/{}:latest".format(native.package_name(), target_basename)
    oci_tarball(
        name = "{}_tarball".format(target_basename),
        image = kwargs.get("image"),
        repo_tags = [local_image_path],
        tags = tags,
    )

    py_binary(
        name = target_basename,
        srcs = ["@enkit//bazel/utils/container:container_pusher.py"],
        main = "@enkit//bazel/utils/container:container_pusher.py",
        data = [
            ":{}_dev_oci_push".format(target_basename),
            ":{}_staging_oci_push".format(target_basename),
            ":{}_tarball".format(target_basename),
        ],
        env = {
            "RUNFILES_ROOT": "enkit/{}".format(native.package_name()),
            "DEV_PUSH_SCRIPT": "push_{}_dev_oci_push.sh".format(target_basename),
            "STAGING_PUSH_SCRIPT": "push_{}_staging_oci_push.sh".format(target_basename),
            "NAMESPACE": namespace,
            "REGION": region,
            "PROJECT": project,
            "IMAGE_PATH": image_path,
            "LOCAL_IMAGE_PATH": local_image_path,
            "LOCAL_IMAGE_TARBALL": "{}_tarball/tarball.tar".format(target_basename),
        },
        deps = [
            "@rules_python//python/runfiles",
            "@enkit//bazel/utils/container:exceptions_lib",
            requirement("absl-py"),
            requirement("docker"),
        ],
        tags = tags,
    )

GCP_REGION = "us-docker"
GCP_PROJECT = "enfabrica-container-images"

def _container_repo(ctx):
    repository = "{}.pkg.dev/{}/{}-{}/{}".format(
        ctx.attr.region,
        ctx.attr.project,
        ctx.attr.namespace,
        ctx.attr.repo_type,
        ctx.attr.image_path,
    )
    script = ctx.actions.declare_file("{}_repo.txt".format(ctx.attr.name))
    ctx.actions.write(script, repository)
    return [DefaultInfo(files = depset([script]))]

container_repo = rule(
    implementation = _container_repo,
    attrs = {
        "image_path": attr.string(
            doc = "Path to the container image in the remote repository.",
            mandatory = True,
        ),
        "repo_type": attr.string(
            doc = "Type of container registry repo to push images to.",
            values = ["dev", "staging", "prod"],
            mandatory = True,
        ),
        "namespace": attr.string(
            doc = "Namespace to prefix the container registry repo which is normally the team name.",
            mandatory = True,
        ),
        "region": attr.string(
            doc = "GCP region of the container registry.",
            default = GCP_REGION,
        ),
        "project": attr.string(
            doc = "GCP project of the container registry.",
            default = GCP_PROJECT,
        ),
    },
)
