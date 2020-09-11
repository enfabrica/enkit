package main

import (
	acommands "github.com/enfabrica/enkit/astore/client/commands"
	bcommands "github.com/enfabrica/enkit/lib/client/commands"
	tcommands "github.com/enfabrica/enkit/proxy/ptunnel/commands"

	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/spf13/cobra"

	"github.com/enfabrica/enkit/lib/srand"
	"math/rand"
)

func main() {
	rng := rand.New(srand.Source)

	root := &cobra.Command{
		Use:           "enkit",
		Long:          `enkit - a single command for all your corp and build needs`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example:       `  $ enkit astore push`,
	}

	base := bcommands.NewBase()
	base.Register(root.PersistentFlags())

	login := bcommands.NewLogin(base, "enkit", rng)
	root.AddCommand(login.Command)

	astore := acommands.New(rng, base)
	root.AddCommand(astore.Command)

	tunnel := tcommands.New(base)
	root.AddCommand(tunnel.Command)

	kcobra.RunWithDefaults(root, &base.Populator, &base.Log)
}
