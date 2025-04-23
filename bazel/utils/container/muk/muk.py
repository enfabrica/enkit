"""Helper functions for muk tool"""

# standard libraries
import io
import logging as log
import pathlib
import subprocess
import textwrap

# third party libraries
import docker
from google.protobuf import json_format

# enfabrica libraries
from bazel.utils.container.exceptions import UnofficialBuildException
from bazel.utils.container.muk import muk_pb2 as mpb


def get_repo(repo_file) -> str:
    # Return the first stripped line of the file
    with open(repo_file, "r", encoding="utf-8") as fd:
        return fd.readlines()[0].strip()


# TODO(bbhuynh): deduplicate functions and conslidate between container_pusher and muk
def promote_image(docker_client, staging_image: str, prod_image: str) -> None:
    staging_image = docker_client.images.pull(staging_image)
    staging_image.tag(prod_image)
    digest = ""
    for line in docker_client.images.push(prod_image, stream=True, decode=True):
        if "aux" in line and "Digest" in line.get("aux", {}):
            digest = line.get("aux").get("Digest")
    return digest


def validate_image(image: docker.models.images.Image) -> None:
    # data types are not preserved in docker labels
    if image.labels.get("OFFICIAL_BUILD", "False") == "False":
        raise UnofficialBuildException(image.id)


def push_image(docker_client, image: docker.models.images.Image, repo: str) -> None:
    image.tag(repo)
    docker_client.images.push(repo)


def parse_image_build_def(path: pathlib.Path) -> mpb.ImageBuild:
    with open(path, "r", encoding="utf-8") as f:
        return json_format.Parse(f.read(), mpb.ImageBuild())


def download_astore_files(build_def: mpb.ImageBuild, build_dir: pathlib.Path) -> None:
    for astore_file in build_def.astore_files:
        target = build_dir / astore_file.filename
        log.info("Downloading astore file %s to %s...", astore_file.uid, target)
        subprocess.run(
            ["enkit", "astore", "download", "-o", target, "-u", astore_file.uid],
            check=True,
        )


def generate_dockerfile(
    build_def: mpb.ImageBuild, buf: io.TextIOBase, labels=None
) -> None:
    body = [
        f"FROM {build_def.base_image}\n",
        # TODO(scott): If we have non-astore files to add, they will end up
        # in the astore dir
        "COPY . /astore\n",
    ] + [_run_cmd_from_action(a, build_def) for a in build_def.actions]

    if build_def.HasField("user"):
        body += [f"USER {build_def.user.uid}:{build_def.user.gid}\n"]

    if build_def.distro == mpb.Distro.UBUNTU:
        body += [
            # Clean up after any apt steps
            "RUN apt-get autoclean -y\n",
            'RUN rm -rf /var/cache/apt/* /var/lib/apt/lists/* "${HOME}/.cache" /tmp/*\n',
        ]
    elif build_def.distro == mpb.Distro.CENTOS or build_def.distro == mpb.Distro.ROCKY:
        body += [
            "RUN yum clean all\n",
            "RUN rm -rf ${HOME}/.cache /tmp/*\n",
        ]
    else:
        log.error(f"Unsupported OS distro: {build_def.distro}")

    if labels:
        with open(labels, "r", encoding="utf-8") as fd:
            for line in fd.readlines():
                body += [f"LABEL {line}"]

    # Clean up astore files
    body += ["RUN rm -rf /astore\n"]
    buf.writelines(body)


def _apt_repo_option_format(option: mpb.AptRepoOption) -> str:
    values_str = ",".join(option.values)
    return f"{option.name}={values_str}"


def _run_cmd_from_action(action: mpb.Action, build_def: mpb.ImageBuild) -> str:
    # TODO: Replace with match/case after moving to Python 3.10+
    if action.WhichOneof("action") == "command":
        return f"RUN {action.command.command}\n"
    elif action.WhichOneof("action") == "apt_install":
        run_cmd = """\
RUN DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get update && \\
    DEBIAN_FRONTEND='noninteractive' TZ=UTC apt-get -f install -y --no-install-recommends \\
    {}
        """.format(
            " \\\n    ".join(action.apt_install.packages)
        )
        return textwrap.dedent(run_cmd).strip() + "\n"
    elif action.WhichOneof("action") == "dpkg_add_arch":
        return f"RUN dpkg --add-architecture {action.dpkg_add_arch.architecture}\n"
    elif action.WhichOneof("action") == "apt_add_repo":
        cmd = ""
        aar = action.apt_add_repo
        source_path = f"/etc/apt/sources.list.d/{aar.name}.list"

        options_str = ""
        if len(aar.repo_options):
            options_str = " " + " ".join(
                _apt_repo_option_format(a) for a in aar.repo_options
            )

        components_str = ""
        if len(aar.components):
            components_str = " " + " ".join(aar.components)

        if aar.HasField("signing_key"):
            keyring_path = f"/usr/share/keyrings/{aar.signing_key.name}.gpg"
            cmd += f"RUN curl -fsSL '{aar.signing_key.url}' | apt-key --keyring '{keyring_path}' add -\n"
            cmd += f"RUN echo 'deb [signed-by={keyring_path}{options_str}] {aar.binary_url} {aar.distribution}{components_str}' | tee -a {source_path}\n"
        elif options_str:
            cmd += f"RUN echo 'deb [{options_str}] {aar.binary_url} {aar.distribution}{components_str}' | tee -a {source_path}\n"
        else:
            cmd += f"RUN echo 'deb {aar.binary_url} {aar.distribution}{components_str}' | tee -a {source_path}\n"
        return cmd
    elif action.WhichOneof("action") == "apt_add_repo_backport":
        aard = action.apt_add_repo_backport
        components_str = ""
        if len(aard.components):
            components_str += " ".join(list(aard.components))
        return f"RUN add-apt-repository 'deb {aard.repo_url} {components_str}'\n"
    elif action.WhichOneof("action") == "apt_add_ppa":
        aap = action.apt_add_ppa
        return f"RUN apt-add-repository ppa:{aap.name}\n"
    elif action.WhichOneof("action") == "add_yum_repo":
        add_yum_repo = action.add_yum_repo
        cmd = ""

        cmd += f"RUN yum-config-manager --add-repo={add_yum_repo.base_url}\n"
        if add_yum_repo.HasField("gpgkey"):
            cmd += f"RUN rpm --import {add_yum_repo.gpgkey}\n"
        return cmd
    elif action.WhichOneof("action") == "yum_install":
        yum_install = action.yum_install
        if build_def.distro == mpb.Distro.ROCKY:
            run_cmd = """\
RUN TZ=UTC yum update -y && \\
    TZ=UTC yum install --allowerasing --enablerepo=devel -y \\
    {}""".format(
                " \\\n    ".join(yum_install.packages)
            )
        else:
            run_cmd = """\
RUN TZ=UTC yum update -y && \\
    TZ=UTC yum install -y \\
    {}""".format(
                " \\\n    ".join(yum_install.packages)
            )
        if len(yum_install.rpms):
            run_cmd += """ && \\
    yum localinstall -y \\
    {}
        """.format(
                " \\\n    ".join(yum_install.rpms)
            )
        return textwrap.dedent(run_cmd).strip() + "\n"
    else:
        log.warning("Unsupported action type: %s", action.WhichOneof("action"))


def docker_build(docker_client, build_dir: pathlib.Path, image_tag: str) -> None:
    try:
        image, _ = docker_client.images.build(
            path=str(build_dir),
            tag=image_tag,
            rm=True,
            pull=True,
            forcerm=True,
            squash=True,
        )
    except docker.errors.BuildError as e:
        log.error("Docker image build failed:")
        for line in e.build_log:
            if "stream" in line:
                log.error(line["stream"].strip())
        raise
    return image


def has_https_apt_key_fetch(build_def: mpb.ImageBuild) -> bool:
    keys = [
        action.apt_add_repo.signing_key
        for action in build_def.actions
        if action.WhichOneof("action") == "apt_add_repo"
        and action.apt_add_repo.HasField("signing_key")
    ]
    return any(k for k in keys if k.url.startswith("https"))


def setup_apt_https(build_def: mpb.ImageBuild) -> mpb.ImageBuild:
    action = mpb.Action(
        apt_install=mpb.AptInstall(
            packages=[
                "apt-utils",
                "curl",
                "gnupg2",
                "software-properties-common",
                "apt-transport-https",
            ],
        ),
    )

    build_def.actions.insert(0, action)
    return build_def
