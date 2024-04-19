"""Pushes container images to the dev, staging, and prod container registry"""
# standard libraries
import os
import logging as log

# third party libraries
import docker
from absl import app, flags
from rules_python.python.runfiles import runfiles

FLAGS = flags.FLAGS
flags.DEFINE_string("dev_script", None, "Script returned by the oci_push rule to push images to the dev repo")
flags.DEFINE_string("staging_script", None, "Script returned by the oci_push rule to push images to the staging repo")
flags.DEFINE_string("image_tarball", None, "Image tarball returned by the oci_tarball rule to validate image tags")
flags.DEFINE_string("namespace", None, "Name of the image repo in Artifact Registry")
flags.DEFINE_string("image_path", None, "Path under the Artifact Registry repo name")
flags.DEFINE_string("project", None, "GCP project name")
flags.DEFINE_string("region", None, "GCP region name")
flags.DEFINE_bool("official", False, "Build and push the container from a clean master branch")
flags.DEFINE_bool("clean_build_check", True, "Build and push the container from a clean master branch")
flags.DEFINE_string("promote", None, "Path to the staging image including the sha256 digest to push to prod")

flags.mark_flag_as_required("dev_script")
flags.mark_flag_as_required("staging_script")
flags.mark_flag_as_required("image_tarball")
flags.mark_flag_as_required("namespace")
flags.mark_flag_as_required("image_path")
flags.mark_flag_as_required("project")
flags.mark_flag_as_required("region")


def validate_image(docker_client, tarball):
    with open(tarball, "rb") as fd:
        image = docker_client.images.load(fd)[0]
        if not image.labels.get("OFFICIAL_BUILD"):
            raise UnofficialBuildException(image.id)


def promote_image(docker_client, staging_image_path, region, project, namespace, suffix):
    staging_image = docker_client.images.pull(staging_image_path)
    prod_image_path = "{}.pkg.dev/{}/{}-prod/{}".format(region, project, namespace, suffix)
    staging_image.tag(prod_image_path)
    docker_client.images.push(prod_image_path)


def container_pusher(docker_client, official, clean_build_check, promote):
    r = runfiles.Create()
    dev_script = r.Rlocation(f"enfabrica/{FLAGS.dev_script}")
    staging_script = r.Rlocation(f"enfabrica/{FLAGS.staging_script}")
    tarball = r.Rlocation(f"enfabrica/{FLAGS.image_tarball}")
    region = FLAGS.region
    project = FLAGS.project
    namespace = FLAGS.namespace
    suffix = FLAGS.image_path

    # push container image to the staging repo
    if official:
        if clean_build_check:
            validate_image(docker_client, tarball)
        log.info(f"Executing {staging_script}")
        os.execvp(staging_script, [staging_script])
    # push container image to the prod repo
    elif promote:
        if clean_build_check:
            validate_image(docker_client, tarball)
        log.info(f"Promoting image {promote} to prod")
        promote_image(docker_client, promote, region, project, namespace, suffix)
    # push container image to the dev repo
    else:
        if clean_build_check:
            validate_image(docker_client, tarball)
        log.info(f"Executing {dev_script}")
        os.execvp(dev_script, [dev_script])


def main(argv):
    del argv
    docker_client = docker.from_env()
    container_pusher(docker_client, FLAGS.official, FLAGS.clean_build_check, FLAGS.promote)


if __name__ == "__main__":
    app.run(main)
