"""Tool for building non-hermetic Docker images."""

# standard libraries
import logging as log
import pathlib
import shutil
import tempfile

# third party libraries
import docker
from absl import app, flags

# enfabrica libraries
from bazel.utils.container.muk import muk

flags.DEFINE_string("image_definition_json", None, "Path to JSON image build definition")
flags.DEFINE_string("labels", None, "Docker labels in the form of key-value pairs delimited by '='")
flags.DEFINE_string("dev_repo", None, "Path to a file that contains the destination dev repo path.")
flags.DEFINE_string("staging_repo", None, "Path to a file that contains the destination staging repo path.")
flags.DEFINE_string("prod_repo", None, "Path to a file that contains the destination prod repo path.")
flags.DEFINE_string("image_tag", "latest", "Image tag to use")
flags.DEFINE_bool("official", False, "Build and push the container from a clean master branch")
flags.DEFINE_bool("clean_build_check", True, "Build and push the container from a clean master branch")
flags.DEFINE_string("promote", None, "Path to the staging image including the sha256 digest to push to prod")
flags.DEFINE_boolean("cleanup", True, "If true, cleanup tempdir after build")

flags.mark_flags_as_required(
    [
        "image_definition_json",
        "dev_repo",
        "staging_repo",
        "prod_repo",
    ]
)

FLAGS = flags.FLAGS


def main(argv):
    del argv
    docker_client = docker.from_env()
    # push container image to the prod repo
    if FLAGS.promote:
        repo = muk.get_repo(FLAGS.prod_repo)
        log.info(f"Promoting {FLAGS.promote} to {repo}")
        if FLAGS.clean_build_check:
            image = docker_client.images.pull(FLAGS.promote)
            muk.validate_image(image)
        digest = muk.promote_image(docker_client, FLAGS.promote, repo)
        log.info(f"Finished promoting {FLAGS.promote} to {repo}@{digest}")
    else:
        build_def = muk.parse_image_build_def(pathlib.Path(FLAGS.image_definition_json))

        if muk.has_https_apt_key_fetch(build_def):
            log.info("Adding apt repo with HTTPS fetch; prepending installation of additional deps")
            muk.setup_apt_https(build_def)

        build_dir = pathlib.Path(tempfile.mkdtemp(prefix="muk_build_"))
        log.info("Performing build in tmpdir: %s", build_dir)

        muk.download_astore_files(build_def, build_dir)
        log.info("Finished downloading files from astore")

        dockerfile = build_dir / "Dockerfile"
        with open(dockerfile, "w", encoding="utf-8") as f:
            muk.generate_dockerfile(build_def, f, FLAGS.labels)
        log.info("Generated Dockerfile")

        log.info("Starting Docker build...")
        repo = muk.get_repo(FLAGS.staging_repo) if FLAGS.official else muk.get_repo(FLAGS.dev_repo)
        image = muk.docker_build(docker_client, build_dir, f"{repo}:{FLAGS.image_tag}")
        log.info("Finished Docker build")

        if FLAGS.cleanup:
            log.info("Cleaning up tmpdir %s", build_dir)
            shutil.rmtree(build_dir)
        else:
            log.info("--cleanup is false; leaving tempdir %s", build_dir)

        log.info("")
        log.info("Successfully built: %s", image.id)
        log.info("")
        # push container image to the staging repo
        if FLAGS.official:
            repo = muk.get_repo(FLAGS.staging_repo)
            log.info(f"Pushing {image.id} to {repo}:{FLAGS.image_tag}")
            if FLAGS.clean_build_check:
                muk.validate_image(image)
            muk.push_image(docker_client, image, repo)
            log.info(f"Finished pushing {image.id} to {repo}:{FLAGS.image_tag}")
        # push container image to the dev repo
        else:
            repo = muk.get_repo(FLAGS.dev_repo)
            log.info(f"Pushing {image.id} to {repo}:{FLAGS.image_tag}")
            if FLAGS.clean_build_check:
                muk.validate_image(image)
            muk.push_image(docker_client, image, repo)
            log.info(f"Finished pushing {image.id} to {repo}:{FLAGS.image_tag}")


if __name__ == "__main__":
    app.run(main)
