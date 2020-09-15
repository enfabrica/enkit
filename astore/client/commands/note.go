package commands

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"strings"
)

type NoteCommand struct {
	*cobra.Command
	root *Root
}

func NewNote(root *Root) *NoteCommand {
	command := &NoteCommand{
		Command: &cobra.Command{
			Use:     "annotate",
			Short:   "Adds a human readable note to an artifact",
			Aliases: []string{"note", "darn", "warn"},
			Example: `  $ astore annotate wusyhsim6h5nhukvu5sejtp7eg6eqdgp ""
    Removes the note associated with artifact uid wusy...gp

$ astore annotate wusyhsim6h5nhukvu5sejtp7eg6eqdgp "Do not use this binary, it is broken"
    Adds the specified note to the binary
`,
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (uc *NoteCommand) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return kflags.NewUsageErrorf("use as 'astore annotate UID message' - the UID of exactly one artifact, followed by the message to associate")
	}

	uid := args[0]
	note := strings.Join(args[1:], " ")

	client, err := uc.root.StoreClient()
	if err != nil {
		return err
	}

	arts, err := client.Note(uid, note)
	if err != nil {
		return err
	}

	uc.root.OutputArtifacts(arts)
	return nil
}
