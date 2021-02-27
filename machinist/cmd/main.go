package main

import (
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/spf13/cobra"
)

func main() {
	command := &cobra.Command{
		Use:   "controller",
		Short: "controller is a server in charge of controlling workers",
	}

	oauthFlags := oauth.DefaultRedirectorFlags()
	oauthFlags.Register(&kcobra.FlagSet{command.Flags()}, "")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	//kcobra.PopulateDefaults(command, os.Args,
	//	kflags.NewAssetAugmenter(&logger.NilLogger{}, "controller", flags.Data),
	//)
	kcobra.Run(command)
}
