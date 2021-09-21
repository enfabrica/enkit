package commands

import (
	// "fmt"
	"github.com/enfabrica/enkit/lib/client"
	// "github.com/enfabrica/enkit/lib/config"
	// "github.com/enfabrica/enkit/lib/config/defcon"
	// "github.com/enfabrica/enkit/lib/kflags"
	// "github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/spf13/cobra"
	// "github.com/spf13/pflag"
	// "os"
	"runtime"
	"strings"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	store *client.ServerFlags
}

func New(base *client.BaseFlags) *Root {
	root := NewRoot(base)

	root.AddCommand(NewDownload(root).Command)
	return root
}

func NewRoot(base *client.BaseFlags) *Root {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "gee",
			Short:         "Simplified version control.",
			SilenceUsage:  true,
			SilenceErrors: true,
			Example: `  $ gee init
        Initialize a new gee workspace.
  
  $ eval "$(gee bash_setup)"
        Initialize your environment for using gee.
  
  $ gcd -b feature1
        Create a new branch and change directory to it.
  
  $ gee commit -a -m "main.go: feature 1 works."
        Commit a change to your local repository.
  
  $ gee help
        To have a nice help screen.`,
			Long: `gee - a git/gh wrapper for version control.`,
		},
		BaseFlags: base,
	}
	return rc
}

type Download struct {
	*cobra.Command
	root *Root

	ForceUid  bool
	ForcePath bool
	Output    string
	Overwrite bool
	Arch      string
	Tag       []string
}

func SystemArch() string {
	return strings.ToLower(runtime.GOARCH + "-" + runtime.GOOS)
}

func NewDownload(root *Root) *Download {
	command := &Download{
		Command: &cobra.Command{
			Use:     "download",
			Short:   "Downloads an artifact",
			Aliases: []string{"down", "get", "pull", "fetch"},
		},
		root: root,
	}
	command.Command.RunE = command.Run

	command.Flags().BoolVarP(&command.ForceUid, "force-uid", "u", false, "The argument specified identifies an uid")
	command.Flags().BoolVarP(&command.ForcePath, "force-path", "p", false, "The argument specified identifies a file path")
	command.Flags().StringVarP(&command.Output, "output", "o", ".", "Where to output the downloaded files. If multiple files are supplied, a directory with this name will be created")
	command.Flags().BoolVarP(&command.Overwrite, "overwrite", "w", false, "Overwrite files that already exist")
	command.Flags().StringArrayVarP(&command.Tag, "tag", "t", []string{"latest"}, "Download artifacts matching the tag specified. More than one tag can be specified")

	return command
}
func (dc *Download) Run(cmd *cobra.Command, args []string) error {
	return nil
}

