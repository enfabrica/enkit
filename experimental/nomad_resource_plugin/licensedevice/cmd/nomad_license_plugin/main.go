// Follows example main:
// https://github.com/hashicorp/nomad-skeleton-device-plugin/blob/31e2e063e167ed4bdbba787b659fac75d4bce659/main.go
package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"

	"github.com/enfabrica/enkit/experimental/nomad_resource_plugin/licensedevice"
)

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	switch len(os.Args) {
	case 1:
		// Default case (Nomad is running the plugin)
		return licensedevice.NewPlugin(l)
	case 4:
		// If additional command-line args are passed, then construct a more-complete
		// plugin. This is useful for manual testing, so that we don't need to init
		// the plugin via SetConfig (and figure out how to MessagePack-encode the
		// config, etc.)
		config := &licensedevice.Config{
			DatabaseConnStr: os.Args[1],
			TableName:       os.Args[2],
			NodeID:          os.Args[3],
		}
		p, err := licensedevice.ConfiguredPlugin(l, config)
		if err != nil {
			l.Error("failed to init plugin: %v", err)
			os.Exit(1)
		}
		return p
	default:
		l.Error("Unexpected number of args (%d)", len(os.Args))
		os.Exit(1)
	}
	return nil
}
