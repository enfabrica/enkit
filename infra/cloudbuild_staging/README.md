# Terraform-controlled Cloud Build

This directory contains an experimental Terraform setup for managing the
`enfabrica-cloud-build-staging` project.

## Getting Started

### Initialization

1. **Install Terraform** by following the [appropriate
   instructions](https://www.terraform.io/downloads). TODO: Include `terraform`
   into the dev container, or via bazel rules somehow.

1. **Initialize Terraform modules**: Terraform implements functionality via
   plugins that are downloaded in a one-time initialization step. Until this is
   integrated into bazel rules, run the init step manually to populate the
   `.terraform/` directory; these files should be `.gitignore`d and not checked
   in.

   ```
   terraform -chdir=infra/cloudbuild_staging init
   ```

1. **gcloud login for applications**: Terraform attempts to pick up gcloud
   credentials via its "application default credentials" mechanism, which
   necessitates a special login command:

   ```
   gcloud auth application-default login
   ```

### Preview Changes

`terraform` can be run in a mode that will show the changes made to the project
before execution.

1. Run:

   ```
   terraform -chdir=infra/cloudbuild_staging plan
   ```

### Execute Changes

1. Run:

   ```
   terraform -chdir=infra/cloudbuild_staging apply

   # Extra configuration due to https://github.com/hashicorp/terraform-provider-google/issues/9883
   gcloud \
     --project=enfabrica-cloud-build-staging \
     beta \
     builds \
     triggers \
     import \
     --source=infra/cloudbuild_staging/enkit_bazel_postsubmit.yml
   ```

## Portions not covered by Terraform

### Project: Enable Billing

Open the
[billing](https://console.cloud.google.com/billing/linkedaccount?project=enfabrica-cloud-build-staging)
page as a billing admin and configure billing for the project.

### Cloud Build: Configure Github Repositories

On the [Manage repositories
page](https://console.cloud.google.com/cloud-build/repos?project=cloud-build-290921):
click `CONNECT REPOSITORY` and follow the workflow.

### Secret: Add Cloud Run user as secret viewer

Context: [These instructions](https://cloud.google.com/build/docs/configuring-notifications/configure-smtp#configuring_email_notifications)

Determine the compute service account user for the project, and then replace the
`--member` flag below:

```
gcloud \
  projects \
  add-iam-policy-binding \
  cloud-build-290921 \
  --member=serviceAccount:757178422572-compute@developer.gserviceaccount.com \
  --role=roles/secretmanager.secretAccessor
```

TODO: This should be defined in Terraform as well, when the cloud-build project
is managed using Terraform.

### Enabling APIs

Various GCP APIs need to be enabled before Terraform can successfully apply:

* Cloud Build
* Compute Engine
* Cloud Scheduler
* TODO: Add more

This might be able to be added to Terraform with the [`google_project_service`
resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/google_project_service).