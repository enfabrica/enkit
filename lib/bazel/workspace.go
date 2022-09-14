// Package bazel provides functions and types for interacting with a bazel
// workspace.
package bazel

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/enfabrica/enkit/lib/logger"
	"github.com/bazelbuild/buildtools/wspace"
)

// Workspace corresponds to a bazel workspace on the filesystem, as defined by
// the presence of a WORKSPACE file.
type Workspace struct {
	root    string // Path to the workspace root on the filesystem
	options *baseOptions

	lock sync.Mutex
	bazelBin  string
	sourceDir string
	outputBaseDir string

	sourceFS fs.FS
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

	w.options.Log.Debugf("Opened bazel workspace at %q", rootPath)
	return w, nil
}

func (w *Workspace) getAndCachePath(path string, dest *string) (string, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if *dest != "" {
		return *dest, nil
	}

	dirname, err := w.Info(ForElement(path))
	if err != nil {
		return "", fmt.Errorf("failed to locate %s: %w", path, err)
	}

	(*dest) = dirname
	return dirname, nil
}

func (w *Workspace) GeneratedFilesDir() (string, error) {
	return w.getAndCachePath("bazel-bin", &w.bazelBin)
}

func (w *Workspace) SourceDir() (string, error) {
	return w.getAndCachePath("workspace", &w.sourceDir)
}

func (w *Workspace) OutputBaseDir() (string, error) {
	return w.getAndCachePath("output_base", &w.outputBaseDir)
}

func (w *Workspace) OpenSource(path string) (fs.File, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.sourceFS == nil {
		// SourceDir() internally grabs the lock.
		// Let's release it temporarily.
		w.lock.Unlock()
		srcdir, err := w.SourceDir()
		w.lock.Lock()
		if err != nil {
			return nil, err
		}

		w.sourceFS = os.DirFS(srcdir)
	}
	return w.sourceFS.Open(path)
}

func (w *Workspace) OutputExternal() (string, error) {
       obase, err := w.OutputBaseDir()
       if err != nil {
		return "", err
	}

	return filepath.Join(obase, "external"), nil
}

// bazelCommand generates an executable command that includes:
// * any startup flags
// * subcommand and subcommand args
// * rooted to the correct workspace directory
func (w *Workspace) bazelCommand(subCmd subcommand) (Command, error) {
	args := w.options.Args()
	args = append(args, subCmd.Args()...)
	cmd := exec.Command("bazel", args...)
	cmd.Dir = w.root
	cmd.Env = os.Environ()
	bazelCmd, err := NewCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to construct bazel command: %w", err)
	}
	return bazelCmd, nil
}

func (w *Workspace) Info(options ...InfoOption) (string, error) {
	infoOpts := &infoOptions{}
	InfoOptions(options).apply(infoOpts)

	cmd, err := w.bazelCommand(infoOpts)
	if err != nil {
		return "", err
	}
	defer cmd.Close()

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("bazel info failed: %v\n\nbazel stderr:\n\n%s", err, cmd.StderrContents())
	}

	b, err := cmd.StdoutContents()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}
