package licensedevice

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

type Plugin struct {
}

func NewPlugin() *Plugin {
	return &Plugin{}
}

func (p *Plugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return nil, fmt.Errorf("PluginInfo not implemented")
}

func (p *Plugin) ConfigSchema() (*hclspec.Spec, error) {
	return nil, fmt.Errorf("ConfigSchema not implemented")
}

func (p *Plugin) SetConfig(c *base.Config) error {
	return fmt.Errorf("SetConfig not implemented")
}

func (p *Plugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	return nil, fmt.Errorf("Fingerprint not implemented")
}

func (p *Plugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	return nil, fmt.Errorf("Reserve not implemented")
}

func (p *Plugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	return nil, fmt.Errorf("Stats not implemented")
}
