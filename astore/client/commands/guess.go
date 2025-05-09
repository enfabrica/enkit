package commands

import (
	"fmt"

	"github.com/enfabrica/enkit/astore/client/astore"
	"github.com/enfabrica/enkit/lib/kflags"

	"github.com/spf13/cobra"
)

type Remote struct {
	*cobra.Command

	root *Root
	Suggest SuggestFlags
}

func NewRemote(root *Root) *Remote {
	command := &Remote{
		Command: &cobra.Command{
			Use:     "remote",
			Short:   "Guesses the remote name that will be used for a file",
			Aliases: []string{"guess", "file"},
		},
		root: root,
	}
	command.Command.RunE = command.Run
	command.Suggest.Register(command.Flags())

	return command
}

func (uc *Remote) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kflags.NewUsageErrorf("use as 'astore guess remote <file>...' - one or more files to guess the architecture of")
	}

	for _, arg := range args {
		local, remote, err := astore.SuggestRemote(arg, *uc.Suggest.Options())
		if err != nil {
			uc.root.Log.Errorf("%s: error - %s\n", arg, err)
		} else {
			// TODO(scott): Get this command to obey --console-format flag
			fmt.Printf("%s: %s %s\n", arg, local, remote)
		}
	}

	return nil
}

type Arch struct {
	*cobra.Command
	root *Root
}

func NewArch(root *Root) *Arch {
	command := &Arch{
		Command: &cobra.Command{
			Use:     "arch",
			Short:   "Guesses the architecture of an artifact",
			Aliases: []string{"guess", "file"},
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (uc *Arch) Run(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return kflags.NewUsageErrorf("use as 'astore guess arch <file>...' - one or more files to guess the architecture of")
	}

	for _, arg := range args {
		arch, err := astore.GuessArchOS(arg)
		if err != nil {
			uc.root.Log.Errorf("%s: error - %s\n", arg, err)
			continue
		}

		for _, a := range arch {
			// TODO(scott): Get this command to obey --console-format flag
			fmt.Printf("%s: %s %s\n", arg, a.Cpu, a.Os)
		}
	}

	return nil
}

type Guess struct {
	*cobra.Command
}

func NewGuess(root *Root) *Guess {
	command := &Guess{
		Command: &cobra.Command{
			Use:     "guess",
			Short:   "Uses astore heuristics to guess file names and architecture",
			Aliases: []string{"guess", "suggest", "inspect"},
		},
	}

	command.Command.AddCommand(NewArch(root).Command)
	command.Command.AddCommand(NewRemote(root).Command)

	return command
}
