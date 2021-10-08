// Package bazel provides functions and types for interacting with a bazel
// workspace.
package bazel

import (
	"os/exec"
)

// Workspace corresponds to a bazel workspace on the filesystem, as defined by
// the presence of a WORKSPACE file.
type Workspace struct {
	root    string // Path to the workspace root on the filesystem
	options *baseOptions
}

// OpenWorkspace returns the bazel workspace at the specified path. If
// outputBase is provided, --output_base will be provided to all commands, which
// can allow for caching of bazel data when temp workspaces are used.
func OpenWorkspace(rootPath string, options ...BaseOption) (*Workspace, error) {
	opts := &baseOptions{}
	BaseOptions(options).apply(opts)
	return &Workspace{
		root:    rootPath,
		options: opts,
	}, nil
}

// bazelCommand generates an executable command that includes:
// * any startup flags
// * subcommand and subcommand args
// * rooted to the correct workspace directory
func (w *Workspace) bazelCommand(subCmd subcommand) *exec.Cmd {
	args := w.options.flags()
	args = append(args, subCmd.Args()...)
	cmd := exec.Command("bazel", args...)
	cmd.Dir = w.root
	return cmd
}
