package sinks

import (
	"context"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/google/go-metrics-stackdriver"
	gometrics "github.com/hashicorp/go-metrics"
)

func NewStackdriver(ctx context.Context, gcpProject string) (gometrics.MetricSink, error) {
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP Cloud Monitoring client: %w", err)
	}
	ss := stackdriver.NewSink(client, &stackdriver.Config{
		ProjectID:         gcpProject,
		ReportingInterval: 60 * time.Second,
	})
	return ss, nil
}
