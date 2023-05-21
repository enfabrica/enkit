{
  gcpProjectId: 'example-project-id',
  scrapeConfigs: [
    {
      endpoint: 'http://prometheus.example.com',
      metrics: [
        {
          name: 'ideal_worker_count',
          query: 'quantile_over_time(0.95,(sum(buildbarn_builder_in_memory_build_queue_tasks_scheduled_total{k8s_namespace="buildbarn"}) by (instance_name_prefix, platform, size_class) - sum(buildbarn_builder_in_memory_build_queue_tasks_executing_duration_seconds_count{k8s_namespace="buildbarn"}) by (instance_name_prefix, platform, size_class))[4h:])',
          scrapeInterval: '60s',
          scrapeTimeout: '5s',
          extraLabels: {
            environment: 'prod',
          },
        },
        {
          name: 'ideal_worker_count',
          query: 'quantile_over_time(0.95,(sum(buildbarn_builder_in_memory_build_queue_tasks_scheduled_total{k8s_namespace="buildbarn_staging"}) by (instance_name_prefix, platform, size_class) - sum(buildbarn_builder_in_memory_build_queue_tasks_executing_duration_seconds_count{k8s_namespace="buildbarn_staging"}) by (instance_name_prefix, platform, size_class))[4h:])',
          scrapeInterval: '60s',
          scrapeTimeout: '5s',
          extraLabels: {
            environment: 'staging',
          },
        },
      ],
    },
  ],
}
