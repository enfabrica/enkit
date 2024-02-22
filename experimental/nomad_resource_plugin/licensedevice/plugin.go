package licensedevice

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/docker"
	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/sqldb"
	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
	"github.com/enfabrica/enkit/lib/str"
)

var metricPluginCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "licensedevice",
	Subsystem: "plugin",
	Name:      "results",
	Help:      "The number of times sql has succeeded or errored in various sections of the code",
},
	[]string{
		"location",
		"outcome",
	})

type Plugin struct {
	Log hclog.Logger

	reserver          types.Reserver
	globalUpdater     types.Notifier
	localUpdater      types.Notifier
	licenseHandleRoot string
	nodeID            string
}

type Config struct {
	DatabaseConnStr string `codec:"database_connection_string"`
	TableName       string `codec:"database_table_name"`
	NodeID          string `codec:"node_id"`
}

func NewPlugin(l hclog.Logger) *Plugin {
	return &Plugin{Log: l}
}

func ConfiguredPlugin(l hclog.Logger, config *Config) (*Plugin, error) {
	p := &Plugin{Log: l}
	if err := p.configure(config); err != nil {
		metricPluginCounter.WithLabelValues("ConfiguredPlugin", "error_configure").Inc()
		return nil, fmt.Errorf("failed to configure plugin: %w", err)
	}
	return p, nil
}

func (p *Plugin) PluginInfo() (*base.PluginInfoResponse, error) {
	// Definition: https://github.com/hashicorp/nomad/blob/ff928a804590611111763632388161dc711adf88/plugins/base/base.go#L31
	return &base.PluginInfoResponse{
		Name:              "LicenseDevice",
		Type:              base.PluginTypeDevice,
		PluginApiVersions: []string{device.ApiVersion010},
		PluginVersion:     "v0.1.0",
	}, nil
}

func (p *Plugin) ConfigSchema() (*hclspec.Spec, error) {
	metricPluginCounter.WithLabelValues("ConfigSchema", "ok").Inc()
	return hclspec.NewObject(map[string]*hclspec.Spec{
		"database_connection_string": hclspec.NewAttr("database_connection_string", "string", true),
		"database_table_name": hclspec.NewDefault(
			hclspec.NewAttr("database_table_name", "string", true),
			hclspec.NewLiteral(`"license_status"`),
		),
		"node_id": hclspec.NewAttr("node_id", "string", true),
	}), nil
}

func (p *Plugin) SetConfig(c *base.Config) error {
	config := &Config{}
	if err := base.MsgPackDecode(c.PluginConfig, config); err != nil {
		metricPluginCounter.WithLabelValues("SetConfig", "error_msgpackdecode").Inc()
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	metricPluginCounter.WithLabelValues("SetConfig", "ok").Inc()
	return p.configure(config)
}

func (p *Plugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	p.Log.Info("Fingerprint() called")
	if p.globalUpdater == nil {
		metricPluginCounter.WithLabelValues("Fingerprint", "error_global_updater").Inc()
		return nil, fmt.Errorf("plugin is not configured: nil notifier")
	}

	p.Log.Debug("acquiring global updates channel")
	notifyChan := p.globalUpdater.Chan(ctx)
	p.Log.Debug("acquired global updates channel")
	resChan := make(chan *device.FingerprintResponse)
	go p.fingerprintLoop(ctx, notifyChan, resChan)
	metricPluginCounter.WithLabelValues("Fingerprint", "ok").Inc()
	return resChan, nil
}

func (p *Plugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	p.Log.Info("Reserve() called")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	licenses, err := p.reserver.Reserve(ctx, deviceIDs, p.nodeID)
	if err != nil {
		metricPluginCounter.WithLabelValues("Reserve", "error_reserve").Inc()
		return nil, fmt.Errorf("failed to reserve %v: %w", deviceIDs, err)
	}

	cr := &device.ContainerReservation{}

	var licenseString string
	for _, l := range licenses {
		if licenseString != "" {
			licenseString += ","
		}
		licenseString += l.ID
	}
	cr.Envs = make(map[string]string)
	cr.Envs[docker.LicenseEnvVar] = licenseString
	metricPluginCounter.WithLabelValues("Reserve", "ok").Inc()
	return cr, nil
}

func (p *Plugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	return nil, fmt.Errorf("Stats not implemented")
}

func (p *Plugin) fingerprintLoop(ctx context.Context, notifyChan chan struct{}, resChan chan<- *device.FingerprintResponse) {
	p.Log.Debug("starting fingerprint response loop")

	// Ensure that an initial state is sent without having to wait for external
	// state to change
	go func() { notifyChan <- struct{}{} }()

nextNotification:
	for {
		select {
		case <-ctx.Done():
			for {
				resChan <- &device.FingerprintResponse{Error: fmt.Errorf("context canceled")}
			}
			return
		case _, ok := <-notifyChan:
			if !ok {
				return
			}
		}

		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p.Log.Debug("fetching global license state")
		licenses, err := p.globalUpdater.GetCurrent(rctx)
		p.Log.Debug("finished fetching global license state")
		cancel()
		if err != nil {
			metricPluginCounter.WithLabelValues("fingerprintLoop", "error_global_updater_get_current").Inc()
			p.Log.Error("failed to get global license state", "error", err)
			continue nextNotification
		}

		p.Log.Debug("parsing global license state")
		groups, err := deviceGroupsFromLicenses(licenses)
		p.Log.Debug("finished parsing global license state")
		if err != nil {
			metricPluginCounter.WithLabelValues("fingerprintLoop", "error_device_groups_from_licenses").Inc()
			p.Log.Error("failed to parse global license state", "error", err)
			continue nextNotification
		}

		p.Log.Info("sending fingerprint response")
		resChan <- &device.FingerprintResponse{Devices: groups}
	}
}

func (p *Plugin) configure(config *Config) error {
	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if config.NodeID == "" {
		var err error
		config.NodeID, err = os.Hostname()
		if err != nil {
			metricPluginCounter.WithLabelValues("configure", "error_no_nodeid").Inc()
			return fmt.Errorf("no node id, hostname also failed: %w", err)
		}
	}
	table, err := sqldb.OpenTable(rctx, config.DatabaseConnStr, config.TableName, config.NodeID)
	cancel()
	if err != nil {
		metricPluginCounter.WithLabelValues("configure", "error_open_table").Inc()
		return fmt.Errorf("failed to open DB: %w", err)
	}

	dockerClient, err := docker.NewClient(context.Background(), config.NodeID)
	if err != nil {
		metricPluginCounter.WithLabelValues("configure", "error_docker_newclient").Inc()
		return fmt.Errorf("failed to create local notifier: %w", err)
	}

	p.nodeID = config.NodeID
	p.reserver = table
	p.globalUpdater = table
	p.localUpdater = dockerClient

	go p.localUpdatesLoop(context.Background(), p.localUpdater.Chan(context.Background()))

	return nil
}

func (p *Plugin) localUpdatesLoop(ctx context.Context, notifyChan <-chan struct{}) {
	p.Log.Debug("starting local license use monitoring")

nextNotification:
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-notifyChan:
			if !ok {
				return
			}

			p.Log.Debug("got local license use update")

			rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			licenses, err := p.localUpdater.GetCurrent(rctx)
			cancel()
			if err != nil {
				metricPluginCounter.WithLabelValues("localUpdatesLoop", "error_local_updater_get_current").Inc()
				p.Log.Error("failed to get local license state", "error", err)
				continue nextNotification
			}

			rctx, cancel = context.WithTimeout(ctx, 5*time.Second)
			err = p.reserver.UpdateInUse(rctx, licenses)
			cancel()
			if err != nil {
				metricPluginCounter.WithLabelValues("localUpdatesLoop", "error_reserver_update_in_use").Inc()
				p.Log.Error("failed to update global license state with in-use info", "error", err)
				continue nextNotification
			}
		}
	}
}

func deviceGroupsFromLicenses(ls []*types.License) ([]*device.DeviceGroup, error) {
	deviceGroupMap := map[string]*device.DeviceGroup{}

	for _, l := range ls {
		groupName := fmt.Sprintf("%s::%s", l.Vendor, l.Feature)
		group := deviceGroupMap[groupName]
		if group == nil {
			group = &device.DeviceGroup{
				Type:   "flexlm_license",
				Vendor: l.Vendor,
				Name:   l.Feature,
			}
		}

		var healthDesc string
		switch l.Status {
		case "IN_USE":
			healthDesc = fmt.Sprintf("in use by job %s on node %s", str.ValueOrDefault(l.UserProcess, "<no job>"), str.ValueOrDefault(l.UserNode, "<no node>"))
		case "RESERVED":
			healthDesc = fmt.Sprintf("reserved by job %s on node %s", str.ValueOrDefault(l.UserProcess, "<no job>"), str.ValueOrDefault(l.UserNode, "<no node>"))
		case "FREE":
			healthDesc = ""
		default:
			metricPluginCounter.WithLabelValues("deviceGroupsFromLicenses", "error_incorrect_status").Inc()
			slog.Error("Error, incorrect license state", "state", l.Status)
		}

		device := &device.Device{
			ID:         l.ID,
			Healthy:    l.Status == "FREE",
			HealthDesc: healthDesc,
		}

		group.Devices = append(group.Devices, device)
		deviceGroupMap[groupName] = group
	}

	groups := []*device.DeviceGroup{}
	for _, g := range deviceGroupMap {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Vendor != groups[j].Vendor {
			return groups[i].Vendor < groups[j].Vendor
		}
		return groups[i].Name < groups[j].Name
	})

	return groups, nil
}
