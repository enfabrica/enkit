variable "project_id" {
  type = string
  default = "enfabrica-cloud-build-staging"
}

################################################################################
# ONE-TIME SETUP
# The following resources need to only be configured once for all builds.
################################################################################

# Grab a handle to the existing compute service account
data "google_compute_default_service_account" "default" {
  project = var.project_id
}

# Grab a handle to the existing cloud build service account
resource "google_project_service_identity" "cloud_build" {
  provider = google-beta
  project = var.project_id
  service = "cloudbuild.googleapis.com"
}

# Grab a handle to the existing pubsub service account
resource "google_project_service_identity" "pubsub" {
  provider = google-beta
  project = var.project_id
  service = "pubsub.googleapis.com"
}

# Create a service account that will be used to kick a Cloud Run instance from a
# PubSub message
resource "google_service_account" "cloud_run_pubsub_invoker" {
  project = var.project_id
  account_id = "cloud-run-pubsub-invoker"
  display_name = "Cloud Run Pub/Sub Invoker"
}

# Create an AppEngine app in the desired location
#
# Cloud Scheduler requires an AppEngine app in the same region, due to a
# limitation of the API: https://cloud.google.com/scheduler/docs/#supported_regions
resource "google_app_engine_application" "app" {
  project = var.project_id
  location_id = "us-west1"
}

# Give the cloud build service account read access to devops project Docker
# images
resource "google_storage_bucket_iam_member" "service_account_docker_image_viewer" {
  bucket = "artifacts.devops-284019.appspot.com"
  role = "roles/storage.objectViewer"
  member = "serviceAccount:${google_project_service_identity.cloud_build.email}"
}

# Give the pubsub service account the ability to create service account tokens
resource "google_project_iam_member" "pubsub_auth_token_creator" {
  project = var.project_id
  role = "roles/iam.serviceAccountTokenCreator"
  member = "serviceAccount:${google_project_service_identity.pubsub.email}"
}

# Give the pubsub invoker service account the ability to trigger Cloud Run
# instances
resource "google_cloud_run_service_iam_binding" "cloud_run_pubsub_invoker_is_invoker" {
  location = google_cloud_run_service.smtp_notifier.location
  project = google_cloud_run_service.smtp_notifier.project
  service = google_cloud_run_service.smtp_notifier.name
  role = "roles/run.invoker"
  members = [
    "serviceAccount:${google_service_account.cloud_run_pubsub_invoker.email}",
  ]
}

# Create a GCS bucket for SMTP notifier configs
resource "google_storage_bucket" "configs" {
  project = var.project_id
  name = "enfabrica-cloud-build-staging-configs"
  location = "US"
  uniform_bucket_level_access = true
}

# Give the compute service account read access to the bucket for SMTP notifier
# configs
resource "google_storage_bucket_iam_binding" "configs_bucket_binding" {
  bucket = google_storage_bucket.configs.name
  role = "roles/storage.objectViewer"
  members = [
    "serviceAccount:${data.google_compute_default_service_account.default.email}",
  ]
}

# Create a pubsub topic for cloud build results
resource "google_pubsub_topic" "cloud_builds" {
  project = var.project_id
  name = "cloud-builds"
  message_retention_duration = "86400s" # 24h
}

################################################################################
# PER-BUILD SETUP
# The following resources need to be defined N times - one per build
################################################################################

# Create a Cloud Build build trigger for this build.
#
# TODO: We can't actually express in terraform what is needed to configure the
# trigger properly until
# https://github.com/hashicorp/terraform-provider-google/issues/9883 is
# resolved. Until then, we'll have to emit the corresponding gcloud yaml and
# import via the gcloud CLI:
#
# gcloud \
#   --project=enfabrica-cloud-build-staging \
#   beta \
#   builds \
#   triggers \
#   import \
#   --source=infra/cloudbuild_staging/enkit_bazel_postsubmit.yml
#
# To generate this yaml:
# 
# 1. Run `terraform apply` to create a broken build trigger
# 2. Run `gcloud --project=enfabrica-cloud-build-staging beta builds triggers describe <TRIGGER NAME>` > infra/cloudbuild_staging/TRIGGER_NAME.yml
# 3. Modify the YAML to match the same structure as //infra/cloudbuild_staging/enkit_bazel_postsubmit.yml
resource "google_cloudbuild_trigger" "build-trigger" {
  trigger_template {
    branch_name = "master"
    repo_name = "enfabrica/enkit"
  }
  
  name = "enkit-bazel-postsubmit"
  project = var.project_id
  
  build {
    step {
      name = "gcr.io/devops-284019/developer:stable"
      entrypoint = "/usr/bin/bazelisk"
      args = ["build", "//..."]
      env = ["BAZEL_PROFILE=cloudbuild"]
    }
    step {
      name = "gcr.io/devops-284019/developer:stable"
      entrypoint = "/usr/bin/bazelisk"
      args = ["test", "//..."]
      env = ["BAZEL_PROFILE=cloudbuild"]
    }
    timeout = "1200s" # 20m
    options {
      machine_type = "E2_HIGHCPU_8"
    }
  }
}

# Create a Cloud Scheduler cron-based trigger for this build
resource "google_cloud_scheduler_job" "job" {
  name = "enkit-bazel-postsubmit-cron"
  project = var.project_id
  region = google_app_engine_application.app.location_id
  schedule = "30 4 * * *" # 4:30AM
  #time_zone = "America/Los_Angeles"
  attempt_deadline = "15s"

  http_target {
    http_method = "POST"
    body = base64encode("{\"branchName\": \"master\"}")
    uri = "https://cloudbuild.googleapis.com/v1/projects/${var.project_id}/triggers/${google_cloudbuild_trigger.build-trigger.trigger_id}:run"
    oauth_token {
      scope = "https://www.googleapis.com/auth/cloud-platform"
      service_account_email = data.google_compute_default_service_account.default.email
    }
  }
}

# Create a pubsub subscription to kick the correct SMTP notifier for this build
resource "google_pubsub_subscription" "cloud_builds_smtp_notifier" {
  name = "cloud_builds_smtp_notifier"
  project = var.project_id
  topic = google_pubsub_topic.cloud_builds.name
  push_config {
    push_endpoint = google_cloud_run_service.smtp_notifier.status[0].url
    oidc_token {
      service_account_email = google_service_account.cloud_run_pubsub_invoker.email
    }
  }
}

# Create an SMTP notifier config for this build
resource "google_storage_bucket_object" "smtp_notifier_config" {
  bucket = google_storage_bucket.configs.name
  name = "smtp_notifier/${google_cloudbuild_trigger.build-trigger.name}.yml"
  content = <<EOF
apiVersion: cloud-build-notifiers/v1
kind: SMTPNotifier
metadata:
  name: example-smtp-notifier
spec:
  notification:
    filter: build.status in [Build.Status.FAILURE, Build.Status.TIMEOUT] && build.build_trigger_id == "${google_cloudbuild_trigger.build-trigger.trigger_id}"
    delivery:
      server: smtp.gmail.com
      port: '587'
      sender: bot@enfabrica.net
      from: bot@enfabrica.net
      recipients:
        - scott@enfabrica.net
      password:
        secretRef: bot-password
  secrets:
  - name: bot-password
    value: projects/496137108493/secrets/bot-gsuite-password/versions/1
EOF
  }

# Create an SMTP notifier for this build
resource "google_cloud_run_service" "smtp_notifier" {
  name = "enkit-bazel-postsubmit-smtp-notifier"
  project = var.project_id
  location = "us-west1"

  template {
    spec {
      containers {
        image = "us-east1-docker.pkg.dev/gcb-release/cloud-build-notifiers/smtp:latest"
        env {
          name = "CONFIG_PATH"
          value = "gs://${google_storage_bucket_object.smtp_notifier_config.bucket}/${google_storage_bucket_object.smtp_notifier_config.name}"
        }
        env {
          name = "PROJECT_ID"
          value = var.project_id
        }
      }
    }
  }
}