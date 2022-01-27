package main

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/proxy/credentials"
	"github.com/enfabrica/enkit/proxy/enproxy"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
)

func main() {
	root := &cobra.Command{
		Use:           "enproxy",
		Long:          `proxy - starts an authenticating proxy`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: `  $ enproxy -c ./mappings.toml
	To start a proxy mapping the urls defined in mappings.toml.`,
	}

	set, populator, runner := kcobra.Runner(root, os.Args)

	rng := rand.New(srand.Source)
	base := client.DefaultBaseFlags(root.Name(), "enproxy")

	flags := enproxy.DefaultFlags()
	flags.Register(set, "")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		ep, err := enproxy.New(rng, enproxy.WithLogging(base.Log), enproxy.FromFlags(flags))
		if err != nil {
			return err
		}

		return ep.Run()
	}

	base.LoadFlagAssets(populator, credentials.Data)
	base.Run(set, populator, runner)
}
