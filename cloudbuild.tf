resource "google_cloudbuild_trigger" "internal_repo_update" {
  name = "Updates-the-enkit-version-in-the-internal-repository"

  description = "Updates the enkit version in the internal repository"

  github {
    owner = "enfabrica"
    name  = "enkit"
    push {
      branch = "^master$"
    }
  }

  build {
    step {
      name       = "gcr.io/devops-284019/developer:stable"
      entrypoint = "bash"
      args = [
        "-c",
        file("${path.module}/cloudbuild.sh")
      ]
      secret_env = ["GH_TOKEN"]
    }
    available_secrets {
      secret_manager {
        version_name = "projects/496137108493/secrets/github-enfabrica-bot-token/versions/latest"
        env          = "GH_TOKEN"
      }
    }

    options {
      machine_type = "E2_HIGHCPU_8"
      env = [
        "ENKIT_OVERRIDE_IDENTITY=@enfabrica.net",
        "BAZEL_PROFILE=cloudbuild"
      ]
    }

    timeout = "600s"

    tags = [
      "terraform-managed"
    ]
  }

  tags = [
    "terragorm-managed"
  ]
}