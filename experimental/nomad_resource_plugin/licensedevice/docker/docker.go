package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

var (
	metricGetCurrentDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "licensedevice",
		Subsystem: "docker",
		Name:      "get_current_duration_seconds",
		Help:      "GetCurrent execution time",
	})
	metricMyLicenses = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "licensedevice",
		Subsystem: "docker",
		Name:      "my_licenses",
		Help:      "How many licenses do I think I currently have",
	})
	metricDockerCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "licensedevice",
		Subsystem: "docker",
		Name:      "results",
		Help:      "The number of times sql has succeeded or errored in various sections of the code",
	},
		[]string{
			"location",
			"outcome",
		})
)

const (
	LicenseEnvVar = "LICENSEPLUGIN_RESERVED_IDS"
)

type Client struct {
	nodeID  string
	docker  *client.Client
	filters []eventFilter

	events chan struct{}
}

func NewClient(ctx context.Context, nodeID string) (*Client, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		metricDockerCounter.WithLabelValues("NewClient", "error_new_client").Inc()
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	c := &Client{
		nodeID: nodeID,
		docker: client,
		filters: []eventFilter{
			typeFilter("container"),
			orFilter(
				statusFilter("start"),
				statusFilter("die"),
			),
		},
		events: make(chan struct{}),
	}
	go c.collectEvents(ctx)

	metricDockerCounter.WithLabelValues("NewClient", "ok").Inc()
	return c, nil
}

func (c *Client) GetCurrent(ctx context.Context) ([]*types.License, error) {
	startTime := time.Now()
	defer metricGetCurrentDuration.Observe(float64(time.Now().Sub(startTime).Seconds()))

	inUse := []*types.License{}
	containers, err := c.docker.ContainerList(ctx, dockertypes.ContainerListOptions{})
	if err != nil {
		metricDockerCounter.WithLabelValues("GetCurrent", "error_container_list").Inc()
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	for _, container := range containers {
		details, err := c.docker.ContainerInspect(ctx, container.ID)
		if err != nil {
			metricDockerCounter.WithLabelValues("GetCurrent", "error_container_inspect").Inc()
			return nil, fmt.Errorf("failed to inspect container %q: %w", container.ID, err)
		}
	nextEnv:
		for _, env := range details.Config.Env {
			kv := strings.SplitN(env, "=", 2)
			if kv[0] != LicenseEnvVar {
				continue nextEnv
			}

			ids := strings.Split(kv[1], ",")
			for _, id := range ids {
				inUse = append(inUse, &types.License{
					ID:       id,
					Status:   "IN_USE",
					UserNode: &c.nodeID,
					// TODO(scott): If the CJ job ID is added as a container label, we can
					// fetch that and propagate it instead
					UserProcess: &container.ID,
				})
			}
		}
	}
	metricMyLicenses.Set(float64(len(inUse)))
	metricDockerCounter.WithLabelValues("GetCurrent", "ok").Inc()
	return inUse, nil
}

func (c *Client) Chan(ctx context.Context) chan struct{} {
	return c.events
}

func (c *Client) collectEvents(ctx context.Context) {
reconnect:
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		eventsChan, errChan := c.docker.Events(ctx, dockertypes.EventsOptions{})

	nextMessage:
		for {
			select {
			case event := <-eventsChan:
				for _, f := range c.filters {
					if !f(event) {
						continue nextMessage
					}
				}
				c.events <- struct{}{}
			case <-errChan:
				time.Sleep(time.Second)
				continue reconnect
			}
		}
	}
}

type eventFilter func(e events.Message) bool

func typeFilter(t string) eventFilter {
	return func(e events.Message) bool {
		return t == e.Type
	}
}

func orFilter(fs ...eventFilter) eventFilter {
	return func(e events.Message) bool {
		for _, f := range fs {
			if f(e) {
				return true
			}
		}
		return false
	}
}

func statusFilter(s string) eventFilter {
	return func(e events.Message) bool {
		return s == e.Status
	}
}
