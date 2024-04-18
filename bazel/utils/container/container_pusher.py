"""Pushes container images to the dev, staging, and prod container registry"""
# standard libraries
import os

# third party libraries
import docker
from absl import app, flags
from rules_python.python.runfiles import runfiles

FLAGS = flags.FLAGS
flags.DEFINE_bool("official", False, "Build and push the container from a clean master branch")
flags.DEFINE_bool("clean_build_check", True, "Build and push the container from a clean master branch")
flags.DEFINE_string("promote", None, "Path to the staging image including the sha256 digest to push to prod")


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
    dev_script = r.Rlocation("{}/{}".format(os.getenv("RUNFILES_ROOT"), os.getenv("DEV_PUSH_SCRIPT")))
    staging_script = r.Rlocation("{}/{}".format(os.getenv("RUNFILES_ROOT"), os.getenv("STAGING_PUSH_SCRIPT")))
    tarball = r.Rlocation("{}/{}".format(os.getenv("RUNFILES_ROOT"), os.getenv("LOCAL_IMAGE_TARBALL")))
    region = os.getenv("REGION")
    project = os.getenv("PROJECT")
    namespace = os.getenv("NAMESPACE")
    suffix = os.getenv("IMAGE_PATH")

    # push container image to the staging repo
    if official:
        if clean_build_check:
            validate_image(docker_client, tarball)
        os.execvp(staging_script, [staging_script])
    # push container image to the prod repo
    elif promote:
        if clean_build_check:
            validate_image(docker_client, tarball)
        promote_image(docker_client, promote, region, project, namespace, suffix)
    # push container image to the dev repo
    else:
        if clean_build_check:
            validate_image(docker_client, tarball)
        os.execvp(dev_script, [dev_script])


def main(argv):
    del argv
    docker_client = docker.from_env()
    container_pusher(docker_client, FLAGS.official, FLAGS.clean_build_check, FLAGS.promote)


if __name__ == "__main__":
    app.run(main)
