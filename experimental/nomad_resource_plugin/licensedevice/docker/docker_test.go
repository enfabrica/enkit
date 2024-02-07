package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice/types"
)

func TestClientIsNotifier(t *testing.T) {
	var notifier *types.Notifier
	assert.Implements(t, notifier, &Client{})
}
