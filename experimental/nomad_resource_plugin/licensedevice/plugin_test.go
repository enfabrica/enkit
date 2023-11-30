package licensedevice

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/plugins/device"
	"github.com/stretchr/testify/assert"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
	"github.com/enfabrica/enkit/lib/str"
)

var sampleLicenseTable = []*types.License{
	{
		ID:      "aaaa",
		Vendor:  "vendor_a",
		Feature: "feature_1",
		Status:  "FREE",
	},
	{
		ID:          "bbbb",
		Vendor:      "vendor_b",
		Feature:     "feature_2",
		Status:      "IN_USE",
		UserNode:    str.Pointer("node-1234"),
		UserProcess: str.Pointer("job-abcd"),
	},
	{
		ID:          "cccc",
		Vendor:      "vendor_c",
		Feature:     "feature_3",
		Status:      "RESERVED",
		UserNode:    str.Pointer("node-2345"),
		UserProcess: str.Pointer("job-bcde"),
	},
}

func TestPluginIsNomadDevicePlugin(t *testing.T) {
	var pluginType *device.DevicePlugin
	assert.Implements(t, pluginType, &Plugin{})
}

func TestPluginFingerprintBeforeSetConfig(t *testing.T) {
	p := NewPlugin()
	_, gotErr := p.Fingerprint(context.Background())

	assert.Error(t, gotErr)
}

func TestPluginFingerprint(t *testing.T) {
	notifier := &mockNotifier{}

	p := NewPlugin()
	p.notifier = notifier

	notifyChan := make(chan struct{})
	notifier.On("Chan").Return(notifyChan)
	notifier.On("GetCurrent").Return(sampleLicenseTable, nil)

	ctx, cancel := context.WithCancel(context.Background())
	gotChan, gotErr := p.Fingerprint(ctx)

	if !assert.NoError(t, gotErr) {
		return
	}

	for i := 0; i < 5; i++ {
		notifyChan <- struct{}{}

		var got *device.FingerprintResponse
		go func() {
			got = <-gotChan
		}()
		if !assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.NotNil(c, got)
		}, 1*time.Second, 10*time.Millisecond, "never got license info") {
			return
		}

		assert.Equal(t, &device.FingerprintResponse{
			Devices: []*device.DeviceGroup{
				{
					Type:   "flexlm_license",
					Vendor: "vendor_a",
					Name:   "feature_1",
					Devices: []*device.Device{
						{
							ID:      "aaaa",
							Healthy: true,
						},
					},
				},
				{
					Type:   "flexlm_license",
					Vendor: "vendor_b",
					Name:   "feature_2",
					Devices: []*device.Device{
						{
							ID:         "bbbb",
							Healthy:    false,
							HealthDesc: "in use by job job-abcd on node node-1234",
						},
					},
				},
				{
					Type:   "flexlm_license",
					Vendor: "vendor_c",
					Name:   "feature_3",
					Devices: []*device.Device{
						{
							ID:         "cccc",
							Healthy:    false,
							HealthDesc: "reserved by job job-bcde on node node-2345",
						},
					},
				},
			},
		}, got)
	}

	cancel()

	var got *device.FingerprintResponse
	go func() {
		got = <-gotChan
	}()
	if !assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NotNil(c, got)
	}, 1*time.Second, 10*time.Millisecond, "never got channel response") {
		return
	}

	assert.Equal(t, &device.FingerprintResponse{Error: fmt.Errorf("context canceled")}, got)
}
