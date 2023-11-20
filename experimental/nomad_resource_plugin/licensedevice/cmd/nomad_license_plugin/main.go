// Follows example main:
// https://github.com/hashicorp/nomad-skeleton-device-plugin/blob/31e2e063e167ed4bdbba787b659fac75d4bce659/main.go
package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice"
)

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &licensedevice.Plugin{Log: l}
}
