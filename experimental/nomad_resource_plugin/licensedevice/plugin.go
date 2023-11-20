package licensedevice

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/sqldb"
	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
	"github.com/enfabrica/enkit/lib/str"
)

type Plugin struct {
	Log hclog.Logger

	reserver types.Reserver
	notifier types.Notifier
}

type Config struct {
	DatabaseConnStr string `codec:"database_connection_string"`
	TableName       string `codec:"database_table_name"`
}

func NewPlugin() *Plugin {
	return &Plugin{}
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
	return hclspec.NewObject(map[string]*hclspec.Spec{
		"database_connection_string": hclspec.NewAttr("database_connection_string", "string", true),
		"database_table_name": hclspec.NewDefault(
			hclspec.NewAttr("database_table_name", "string", true),
			hclspec.NewLiteral(`"license_status"`),
		),
	}), nil
}

func (p *Plugin) SetConfig(c *base.Config) error {
	config := &Config{}
	if err := base.MsgPackDecode(c.PluginConfig, config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	table, err := sqldb.OpenTable(rctx, config.DatabaseConnStr, config.TableName)
	if err != nil {
		return fmt.Errorf("failed to open DB: %w", err)
	}

	p.reserver = table
	p.notifier = table

	return nil
}

func (p *Plugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	if p.notifier == nil {
		return nil, fmt.Errorf("plugin is not configured: nil notifier")
	}

	notifyChan := p.notifier.Chan(ctx)
	resChan := make(chan *device.FingerprintResponse)
	go p.fingerprintLoop(ctx, notifyChan, resChan)

	return resChan, nil
}

func (p *Plugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	return nil, fmt.Errorf("Reserve not implemented")
}

func (p *Plugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	return nil, fmt.Errorf("Stats not implemented")
}

func (p *Plugin) fingerprintLoop(ctx context.Context, notifyChan <-chan struct{}, resChan chan<- *device.FingerprintResponse) {
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
		licenses, err := p.notifier.GetCurrent(rctx)
		cancel()
		if err != nil {
			// TODO: metric, log
		}

		groups, err := deviceGroupsFromLicenses(licenses)
		if err != nil {
			// TODO: metric, log?
		}

		resChan <- &device.FingerprintResponse{Devices: groups}
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
			// TODO: error + metric
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
