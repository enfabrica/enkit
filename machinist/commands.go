package machinist

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/spf13/cobra"
)

type ServerFlagSet struct {
	Port int
}

func NewCommands(baseFlags *client.BaseFlags) *cobra.Command{

	mainCommand := &cobra.Command{

	}
	serveCommand := &cobra.Command{

	}
	joinCommand := &cobra.Command{

	}

	mainCommand.AddCommand(serveCommand)
	mainCommand.AddCommand(joinCommand)
	return nil
}
