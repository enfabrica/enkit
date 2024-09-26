load("//bazel/utils:files.bzl", "write_to_file")
load("//bazel/dive:dive.bzl", "oci_dive")
load("@enkit//bazel/utils:merge_kwargs.bzl", "merge_kwargs")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_push", "oci_load")
load("@enkit_pip_deps//:requirements.bzl", "requirement")
load("@rules_python//python:defs.bzl", "py_binary")

GCP_REGION = "us-docker"
GCP_PROJECT = "enfabrica-container-images"

_IMAGE_BUILDER_SH = """\
#!/bin/bash
{tool} \\
    --image_definition_json={image_def} \\
    --labels={labels} \\
    --dev_repo={dev_repo} \\
    --staging_repo={staging_repo} \\
    --prod_repo={prod_repo} \\
    --v=4 "$@"
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
        transitive_files = ctx.attr._tool[DefaultInfo].default_runfiles.files,
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

def container_dive(*args, **kwargs):
    oci_dive(*args, **kwargs)

def container_image(*args, **kwargs):
    name = kwargs.get("name")
    output = "{}_labels.txt".format(name)
    tags = kwargs.get("tags", [])

    # The 'image' field in oci_pull does not need the //image target
    # while the container_pull rule does. Modify this wrapper script
    # to insert the //image target when using container_pull.
    # Remove once oci_pull doesn't have auth errors anymore.
    #if kwargs.get("base", "").startswith("@"):
    #    kwargs["base"] = "{}//image".format(kwargs.get("base"))

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
    oci_load(*args, **kwargs)

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
#    oci_load(
#        name = "{}_tarball".format(target_basename),
#        image = kwargs.get("image"),
#        repo_tags = [local_image_path],
#        tags = tags,
#    )
#    native.filegroup(
#        name = "{}.tar".format(target_basename),
#        srcs = [":{}_tarball".format(target_basename)],
#        output_group = "tarball",
#    )
    container_pusher(
        name = target_basename,
        dev_script = ":{}_dev_oci_push".format(target_basename),
        staging_script = ":{}_staging_oci_push".format(target_basename),
        image_tarball = kwargs.get("image"),
        namespace = namespace,
        image_path = image_path,
        tags = tags,
    )

def container_pusher_impl(ctx):
    script = ctx.actions.declare_file("{}_push_script.sh".format(ctx.attr.name))
    body = """#!/bin/bash
{} \\
--dev_script {} \\
--staging_script {} \\
--image_tarball {} \\
--namespace {} \\
--image_path {} \\
--project {} \\
--region {} \\
--v=1 \\
$@
""".format(
    ctx.executable._tool.short_path,
    ctx.file.dev_script.short_path,
    ctx.file.staging_script.short_path,
    ctx.file.image_tarball.short_path,
    ctx.attr.namespace,
    ctx.attr.image_path,
    ctx.attr.project,
    ctx.attr.region,
)
    ctx.actions.write(script, body)

    direct_files = ctx.files.dev_script + ctx.files.staging_script + ctx.files.image_tarball
    transitive_files = ctx.attr._tool[DefaultInfo].default_runfiles.files.to_list() + \
        ctx.attr.dev_script[DefaultInfo].default_runfiles.files.to_list() + \
        ctx.attr.staging_script[DefaultInfo].default_runfiles.files.to_list() + \
        ctx.attr.image_tarball[DefaultInfo].default_runfiles.files.to_list()
    runfiles = ctx.runfiles(
        files = ctx.attr._tool[DefaultInfo].files.to_list() + direct_files,
        transitive_files = depset(transitive_files),
    )
    return [
        DefaultInfo(
            runfiles = runfiles,
            executable = script,
        ),
    ]

container_pusher = rule(
    implementation = container_pusher_impl,
    executable = True,
    attrs = {
        "dev_script": attr.label(
            doc = "Script returned by the oci_push rule to push images to the dev repo",
            allow_single_file = [".sh"],
            mandatory = True,
        ),
        "staging_script": attr.label(
            doc = "Script returned by the oci_push rule to push images to the staging repo",
            allow_single_file = [".sh"],
            mandatory = True,
        ),
        "image_tarball": attr.label(
            doc = "Image tarball returned by the oci_load rule to validate image tags",
            allow_single_file = [".tar"],
            mandatory = True,
        ),
        "namespace": attr.string(
            doc = "Name of the image repo in Artifact Registry",
            mandatory = True,
        ),
        "image_path": attr.string(
            doc = "Path under the Artifact Registry repo name",
            mandatory = True,
        ),
        "project": attr.string(
            doc = "GCP project name",
            default = GCP_PROJECT,
        ),
        "region": attr.string(
            doc = "GCP region name",
            default = GCP_REGION,
        ),
        "_tool": attr.label(
            doc = "Container pusher binary",
            default = "//bazel/utils/container:container_pusher",
            executable = True,
            cfg = "exec",
        ),
    },
)

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

def container_bootstrap_rule_impl(ctx):
    outfile = ctx.actions.declare_file("%s.tar" % ctx.attr.name)
    args = ctx.actions.args()
    args.add(ctx.attr.arch)
    args.add(",".join(ctx.attr.components))
    args.add(ctx.attr.distro)
    args.add(ctx.attr.mirror)
    args.add(outfile)
    args.add(ctx.executable.chroot_script)
    args.add_all(ctx.files.pkgs)

    ctx.actions.run(
        executable = ctx.executable.bootstrap_script,
        inputs = ctx.files.pkgs + [ctx.executable.chroot_script],
        outputs = [outfile],
        arguments = [args],
    )

    return [
        DefaultInfo(files = depset([outfile])),
    ]


bootstrap_attrs = {
    "bootstrap_script": attr.label(
        doc = "Script that executes the bootstrap tool",
        allow_single_file = [".sh"],
        executable = True,
        cfg = "exec",
        default = "@enkit//bazel/utils/container:bootstrap_ubuntu.sh",
    ),
    "distro": attr.string(
        doc = "Ubuntu Linux distro to bootstrap the container",
        values = [
            "bionic",
            "focal",
            "jammy",
            "noble",
        ],
        mandatory = True,
    ),
    "components": attr.string_list(
        doc = "Components of the repo found under /etc/apt/sources.list",
        default = [
            "main",
            "restricted",
            "universe",
            "multiverse",
        ],
    ),
    "mirror": attr.string(
        doc = """
URL to https://snapshot.ubuntu.com/ubuntu/<snapshot-time>. By default, use the latest snapshot.
Generate new timestamps with: date -u +"%Y%m%dT%H%M%SZ"
""",
        default = "https://snapshot.ubuntu.com/ubuntu",
    ),
    "arch": attr.string(
        doc = "CPU architecture",
        values = [
            "amd64",
            "i386",
            "arm64",
        ],
        default = "amd64",
    ),
}

container_bootstrap_rule = rule(
    implementation = container_bootstrap_rule_impl,
    attrs = bootstrap_attrs | {
        "pkgs": attr.label_list(
            doc = "List of ubuntu_pkg targets to install",
            allow_files = [".tar"],
            mandatory = True,
        ),
        "chroot_script": attr.label(
            doc = "Script that executes via chroot in the bootstrap env",
            allow_single_file = [".sh"],
            executable = True,
            cfg = "exec",
            default = "@enkit//bazel/utils/container:chroot_ubuntu.sh",
        ),
    }
)

def ubuntu_pkg_rule_impl(ctx):
    outfile = ctx.actions.declare_file("%s.tar" % ctx.attr.name)
    args = ctx.actions.args()
    args.add(ctx.attr.arch)
    args.add(",".join(ctx.attr.components))
    args.add(ctx.attr.pkg)
    args.add(ctx.attr.distro)
    args.add(ctx.attr.mirror)
    args.add(outfile)

    ctx.actions.run(
        executable = ctx.executable.bootstrap_script,
        outputs = [outfile],
        arguments = [args],
    )

    return [
        DefaultInfo(files = depset([outfile])),
    ]

ubuntu_pkg_rule = rule(
    implementation = ubuntu_pkg_rule_impl,
    attrs = bootstrap_attrs | {
        "pkg": attr.string(
            doc = "Name of the package to download",
            mandatory = True,
        )
    }
)

def ubuntu_bootstrap(*args, **kwargs):
    reformatted_targets = []
    for p in kwargs.get("pkgs", []):
        if p.endswith(":i386"):
            reformatted_targets += [p.removesuffix(":i386")]
        else:
            reformatted_targets += [p]
    kwargs["pkgs"] = reformatted_targets
    return container_bootstrap_rule(*args, **kwargs)

def ubuntu_pkg(*args, **kwargs):
    kwargs["bootstrap_script"] = "@enkit//bazel/utils/container:ubuntu_pkg.sh"
    # When installing 32-bit packages, the package name convention is to suffix
    # the package name with :i386. However, colons are not allowed as target names in bazel.
    if kwargs.get("name", "").endswith(":i386"):
        kwargs["name"] = kwargs.get("name", "").removesuffix(":i386")
    return ubuntu_pkg_rule(*args, **kwargs)
