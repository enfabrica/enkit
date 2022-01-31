package kzip

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var runCommand = func(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed. output:\n%s\n%w", string(output), err)
	}
	return nil
}

// TempUnzipDir wraps a path to a temp directory that contains the contents of
// an unzipped archive.
type TempUnzipDir struct {
	tempDir string
}

// Close deletes the underlying temporary directory.
func (d *TempUnzipDir) Close() error {
	err := os.RemoveAll(d.tempDir)
	if err != nil {
		return fmt.Errorf("failed to delete temp zip dir %q: %w", d.tempDir, err)
	}
	return nil
}

// Root returns the path to the temporary directory, which is the root of the
// unzipped archive.
func (d *TempUnzipDir) Root() string {
	return d.tempDir
}

// Path returns an absolute path to a file on disk for a path relative to the
// root of the archive.
func (d *TempUnzipDir) Path(relPath string) string {
	return filepath.Join(d.tempDir, relPath)
}

// Unzip unzips the supplied ZIP file to a temp directory, returning a handle to
// the unzipped archive. This handle should be closed when the file contents are
// no longer in-use.
func Unzip(ctx context.Context, zipPath string) (*TempUnzipDir, error) {
	template := fmt.Sprintf(
		"kzip_%s_*",
		strings.Replace(
			filepath.Base(zipPath),
			".",
			"_",
			-1,
		),
	)
	dir, err := os.MkdirTemp("", template)
	if err != nil {
		return nil, fmt.Errorf("failed to create unzip destination: %w", err)
	}

	command := exec.CommandContext(ctx, "unzip", zipPath, "-d", dir)

	if err := runCommand(command); err != nil {
		return nil, fmt.Errorf("unzip failed: %w", err)
	}

	return &TempUnzipDir{
		tempDir: dir,
	}, nil
}
