#!/bin/bash
pwd
echo relative path
gsutil cp experimental/nomad_resource_plugin/licensedevice/cmd/nomad_license_plugin/nomad_license_plugin_/nomad_license_plugin gs://enfabrica-cluster-prod-nomad-plugin/nomad_license_plugin
gsutil stat gs://enfabrica-cluster-prod-nomad-plugin/nomad_license_plugin

