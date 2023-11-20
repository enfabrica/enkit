package licensedevice

import (
	"testing"
	
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/stretchr/testify/assert"
)

func TestPluginIsNomadDevicePlugin(t *testing.T) {
	var pluginType *device.DevicePlugin
	assert.Implements(t, pluginType, &Plugin{})
}
