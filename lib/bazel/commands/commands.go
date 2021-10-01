package commands

import (
	"fmt"

	"github.com/enfabrica/enkit/lib/client"

	"github.com/spf13/cobra"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags
}

func New(base *client.BaseFlags) *Root {
	root := NewRoot(base)

	root.AddCommand(NewAffectedTargets(root).Command)

	return root
}

func NewRoot(base *client.BaseFlags) *Root {
	rc := &Root {
		Command: &cobra.Command{
			Use: "bazel",
			Short: "Perform bazel helper actions",
			SilenceUsage: true,
			SilenceErrors: true,
			Long: `bazel - performs helper bazel operations`,
		},
		BaseFlags: base,
	}
	return rc
}

type AffectedTargets struct {
	*cobra.Command
	root *Root

	Start string
	End string
}

func NewAffectedTargets(root *Root) *AffectedTargets {
	command := &AffectedTargets{
		Command: &cobra.Command{
			Use: "affected-targets",
			Short: "Operations involving changed bazel targets between two source revision points",
			Aliases: []string{"at"},
		},
		root: root,
	}

	command.PersistentFlags().StringVarP(&command.Start, "start", "s", "HEAD", "Git committish of 'before' revision")
	command.PersistentFlags().StringVarP(&command.End, "end", "e", "", "Git committish of 'end' revision, or empty for current dir with uncomitted changes")

	command.AddCommand(NewAffectedTargetsList(command).Command)

	return command
}

type AffectedTargetsList struct {
	*cobra.Command
	parent *AffectedTargets
}

func NewAffectedTargetsList(parent *AffectedTargets) *AffectedTargetsList {
	command := &AffectedTargetsList {
		Command: &cobra.Command{
			Use: "list",
			Short: "List affected targets between two source revision points",
			Aliases: []string{"ls"},
			Example: `  $ enkit bazel affected-targets list
        List affected targets between the last commit and current uncommitted changes.

  $ enkit bazel affected-targets list --start=HEAD~1 --end=HEAD
        List affected targets in the most recent commit.`,
		},
		parent: parent,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *AffectedTargetsList) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not yet implemented")
}

