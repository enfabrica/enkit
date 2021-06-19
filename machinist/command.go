package machinist

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/spf13/cobra"
)

func NewRootCommand(bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use: "machinist",
	}
	conf := &config.Common{
		Root: bf,
	}
	c.PersistentFlags().StringVar(&conf.ControlPlaneHost, "control-host", "localhost", "")
	c.PersistentFlags().IntVar(&conf.ControlPlanePort, "control-port", 4545, "")
	c.PersistentFlags().IntVar(&conf.MetricsPort, "metrics-port", 9090, "")
	return c
}
