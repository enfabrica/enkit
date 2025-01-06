#!/bin/bash
oci_pull_targets="sonora_base_image \
    dev_base_image \
    hw_dev_base_image \
    puppet_server_base_image \
    golang_oci_base \
    cj_base_image \
    dgxfighter_base_image \
    python_base \
    default-centos \
    default-ubuntu \
    ubuntu-18.04.4 \
    flexlm-exporter \
    prometheus_base \
    grafana_base \
    grafana_oci_base_image \
    prometheus_bq_remote_storage_adapter_base \
    puppet_server \
    airflow_oci_base \
    infra_benchmark_base \
    fuse_base"
for target in $oci_pull_targets
do
    bazel query "rdeps(//..., @$target)" --noimplicit_deps
done
