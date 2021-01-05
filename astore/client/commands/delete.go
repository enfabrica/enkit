package commands

import (
	"github.com/spf13/cobra"
)

type DeleteCommand struct {
	*cobra.Command
	root *Root

	name string
}

func (uc *Delete) Run(cmd *cobra.Command, args []string) error {
	//TODO
	return nil
}

type Delete struct {
	*cobra.Command
	root *Root
	isDryRun bool
}

func NewDelete(root *Root) *Delete {
	command := &Delete{
		Command: &cobra.Command{
			Use:   "delete",
			Short: "Deletes an artifact from astore",

		},
		root: root,
	}
	command.Flags().BoolVarP(&command.isDryRun, "dry-run", "dr",  false, "will attempt to delete the resource without doing so")
	command.Command.RunE = command.Run
	return command
}

