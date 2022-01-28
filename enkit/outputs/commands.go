package outputs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/spf13/cobra"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	OutputsRoot string
}

func New(base *client.BaseFlags) (*Root, error) {
	root, err := NewRoot(base)
	if err != nil {
		return nil, err
	}

	root.AddCommand(NewMount(root).Command)
	root.AddCommand(NewUnmount(root).Command)
	root.AddCommand(NewRun(root).Command)
	root.AddCommand(NewShutdown(root).Command)

	return root, nil
}

func NewRoot(base *client.BaseFlags) (*Root, error) {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "outputs",
			Short:         "Commands for mounting remotely-built Bazel outputs",
			SilenceUsage:  true,
			SilenceErrors: true,
			Long:          `outputs - commands for mounting remotely-built Bazel outputs`,
		},
		BaseFlags: base,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to detect $HOME: %w", err)
	}
	defaultOutputsRoot := filepath.Join(homeDir, "outputs")

	rc.PersistentFlags().StringVar(&rc.OutputsRoot, "outputs_root", defaultOutputsRoot, "Root dir of mounted outputs")
	return rc, nil
}

type Mount struct {
	*cobra.Command
	root *Root
}

func NewMount(root *Root) *Mount {
	command := &Mount{
		Command: &cobra.Command{
			Use:   "mount [invocation ID]",
			Short: "Mount the build outputs of a particular invocation",
			Example: `  $ enkit outputs mount 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Mounts outputs from build 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07 to the
	default location.`,
			Args: cobra.ExactArgs(1),
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *Mount) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("`enkit outputs mount` is unimplemented")
}

type Unmount struct {
	*cobra.Command
	root *Root
}

func NewUnmount(root *Root) *Unmount {
	command := &Unmount{
		Command: &cobra.Command{
			Use:   "unmount [invocation ID]",
			Short: "Unmount the build outputs of a particular invocation",
			Example: `  $ enkit outputs unmount 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Unmounts outputs from build 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07 from the
	default location.`,
			Aliases: []string{"umount"},
			Args:    cobra.ExactArgs(1),
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *Unmount) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("`enkit outputs unmount` is unimplemented")
}

type Run struct {
	*cobra.Command
	root *Root

	InvocationID string
	DirPath      string
	ZipPath      string
}

func NewRun(root *Root) *Run {
	command := &Run{
		Command: &cobra.Command{
			Use:   "run",
			Short: "Mount build artifacts and execute an optional command on them",
			Example: `  $ enkit outputs run --invocation=73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Launches a shell with build outputs from a particular build re-rooted so that
	paths are correct within the outputs themselves.

  $ enkit outputs run --dir=/tmp/some_dir
	Launches a shell with build outputs in /tmp/some_dir re-rooted so that paths
	are correct within the outputs themselves.

  $ enkit outputs run --zip=/tmp/some.zip
	Unpacks the named zip into a tempdir, and then reroots the artifacts so that
	paths are correct within the outputs themselves.

  $ enkit outputs run --zip=/tmp/some-zip -- find .
	Runs "find ." in the unpacked zip after it is rerooted.`,
		},
		root: root,
	}

	command.Command.RunE = command.Run
	command.Command.PreRunE = command.validate
	command.Flags().StringVar(&command.InvocationID, "invocation_id", "", "If set, the build's invocation ID from which to mount artifacts")
	command.Flags().StringVar(&command.DirPath, "dir_path", "", "If set, the path to already-unpacked artifacts to reroot")
	command.Flags().StringVar(&command.ZipPath, "zip_path", "", "If set, the path to a zipped outputs file to unpack and re-root")
	return command
}

func (c *Run) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("`enkit outputs run` is unimplemented")
}

func (c *Run) validate(cmd *cobra.Command, args []string) error {
	setCount := 0
	for _, f := range []string{c.InvocationID, c.DirPath, c.ZipPath} {
		if f != "" {
			setCount++
		}
	}
	switch {
	case setCount == 0:
		return fmt.Errorf("One of --invocation_id, --dir_path, --zip_path must be set")
	case setCount >= 2:
		return fmt.Errorf("Only one of --invocation_id, --dir_path, --zip_path may be set")
	}
	return nil
}

type Shutdown struct {
	*cobra.Command
	root *Root
}

func NewShutdown(root *Root) *Shutdown {
	command := &Shutdown{
		Command: &cobra.Command{
			Use:   "shutdown",
			Short: "Unmount all builds under particular directory",
			Example: `  $ enkit outputs shutdown
	Unmounts bb_clientd, and deletes all links to build artifacts.`,
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *Shutdown) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("`enkit outputs Shutdown` is unimplemented")
}
