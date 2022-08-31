package cmd

import (
	"math/rand"

	acommands "github.com/enfabrica/enkit/astore/client/commands"
	ocommands "github.com/enfabrica/enkit/enkit/outputs"
	vcommands "github.com/enfabrica/enkit/enkit/version"
	bazelcmds "github.com/enfabrica/enkit/lib/bazel/commands"
	"github.com/enfabrica/enkit/lib/client"
	bcommands "github.com/enfabrica/enkit/lib/client/commands"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/srand"
	tcommands "github.com/enfabrica/enkit/proxy/ptunnel/commands"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

type EnkitCommand struct {
	cmd       *cobra.Command
	baseFlags *client.BaseFlags
	flagSet   *kcobra.FlagSet
	populator kflags.Populator
	runner    kflags.Runner
}

func New() (*EnkitCommand, error) {
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

	bazel := bazelcmds.New(base)
	root.AddCommand(bazel.Command)

	versionCmd := vcommands.New(base)
	root.AddCommand(versionCmd.Command)

	outputs, err := ocommands.New(base)
	if err != nil {
		return nil, err
	}
	root.AddCommand(outputs.Command)

	return &EnkitCommand{
		cmd:       root,
		baseFlags: base,
		flagSet:   set,
		populator: populator,
		runner:    runner,
	}, nil
}

func (c *EnkitCommand) Run() {
	c.baseFlags.Run(kcobra.HideFlags(c.flagSet), c.populator, c.runner)
}

func (c *EnkitCommand) GenMarkdownTree(path string) error {
	return doc.GenMarkdownTree(c.cmd, path)
}
