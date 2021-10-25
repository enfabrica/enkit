// Package bazel provides functions and types for interacting with a bazel
// workspace.
package bazel

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/enfabrica/enkit/lib/logger"

	"github.com/bazelbuild/buildtools/wspace"
)

// Workspace corresponds to a bazel workspace on the filesystem, as defined by
// the presence of a WORKSPACE file.
type Workspace struct {
	root    string // Path to the workspace root on the filesystem
	options *baseOptions

	bazelBin  fs.FS
	sourceDir fs.FS
}

// FindRoot returns the path to the bazel workspace root in which `dir`
// resides, or an error if `dir` is not inside a bazel workspace.
func FindRoot(dir string) (string, error) {
	root := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if root != "" {
		return root, nil
	}
	root, _ = wspace.FindWorkspaceRoot(dir)
	return root, nil
}

// OpenWorkspace returns the bazel workspace at the specified path. If
// outputBase is provided, --output_base will be provided to all commands, which
// can allow for caching of bazel data when temp workspaces are used.
func OpenWorkspace(rootPath string, options ...BaseOption) (*Workspace, error) {
	opts := &baseOptions{
		Log: &logger.NilLogger{},
	}
	BaseOptions(options).apply(opts)
	w := &Workspace{
		root:    rootPath,
		options: opts,
	}
	generatedFilesDir, err := w.Info(ForElement("bazel-bin"))
	if err != nil {
		return nil, fmt.Errorf("failed to locate bazel-bin: %w", err)
	}
	sourceDir, err := w.Info(ForElement("workspace"))
	if err != nil {
		return nil, fmt.Errorf("failed to detect execution root: %w", err)
	}
	w.bazelBin = os.DirFS(generatedFilesDir)
	w.sourceDir = os.DirFS(sourceDir)
	w.options.Log.Debugf("Opened bazel workspace at %q", rootPath)
	return w, nil
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

func (w *Workspace) Info(options ...InfoOption) (string, error) {
	infoOpts := &infoOptions{}
	InfoOptions(options).apply(infoOpts)

	cmd := w.bazelCommand(infoOpts)
	return runBazelCommand(cmd)
}
