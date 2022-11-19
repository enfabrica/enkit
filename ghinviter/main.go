package main

import (
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/ghinviter/inviter"
	"github.com/enfabrica/enkit/ghinviter/credentials"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
	"log"
)

func main() {
	command := &cobra.Command{
		Use:   "onboarder",
		Short: "onboarder allows to automatically invite and add users",
	}

	flagSet := &kcobra.FlagSet{command.Flags()}

	flags := inviter.DefaultFlags().Register(flagSet, "")

	rng := rand.New(srand.Source)

	command.RunE = func(cmd *cobra.Command, args[]string) error {
		inv, err := inviter.New(inviter.FromFlags(rng, flags), inviter.WithLogger(&logger.DefaultLogger{Printer: log.Printf}))
		if err != nil {
			return err
		}

		return inv.Run()
	}

	// 1) Mark all pages as requiring the authenticator.
	// 2) 

	kcobra.PopulateDefaults(command, os.Args,
		kflags.NewAssetAugmenter(logger.Nil, "onboarder", credentials.Data),
	)
	kcobra.Run(command)
}
