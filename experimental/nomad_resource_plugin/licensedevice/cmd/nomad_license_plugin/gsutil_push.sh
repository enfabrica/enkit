#!/bin/bash
gsutil cp $1 gs://enfabrica-cluster-prod-nomad-plugin/nomad_license_plugin
gsutil stat gs://enfabrica-cluster-prod-nomad-plugin/nomad_license_plugin

