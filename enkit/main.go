package main

import (
	"github.com/enfabrica/enkit/lib/client"

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

	base := client.DefaultBaseFlags(root.Name(), "enkit")

	set, populator, runner := kcobra.Runner(root, nil, base.IdentityErrorHandler("enkit login"))

	login := bcommands.NewLogin(base, rng, populator)
	root.AddCommand(login.Command)

	astore := acommands.New(base)
	root.AddCommand(astore.Command)

	tunnel := tcommands.NewTunnel(base)
	root.AddCommand(tunnel.Command)

	ssh := tcommands.NewSSH(base)
	root.AddCommand(ssh.Command)

	agentCommand := tcommands.NewAgentCommand(base)
	root.AddCommand(agentCommand)

	base.Run(kcobra.HideFlags(set), populator, runner)
}
