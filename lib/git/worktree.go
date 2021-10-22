package git

import (
	"fmt"
	"io/ioutil"
	"os/exec"
)

// TempWorktree is a handle to a git worktree directory created with a
// reference to an existing git repository.
type TempWorktree struct {
	repoPath     string
	worktreePath string
}

var runCommand = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// NewTempWorktree creates a worktree in a temporary directory for the git
// repository at the specified path with the specified committish (commit hash,
// short commit hash, branch name, etc.). It is designed to be used to
// "checkout" a git repository to a particular point without affecting the main
// worktree, which carries the risk of losing data/work and/or conflicting with
// other processes.
//
// This function exists because go-git does not appear to have the ability to
// generate git worktrees; only traverse them.
func NewTempWorktree(repoPath string, committish string) (*TempWorktree, error) {
	tmpDir, err := ioutil.TempDir("", "git_temp_worktree_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// Command info: https://git-scm.com/docs/git-worktree
	cmd := exec.Command("git", "worktree", "add", "--detach", tmpDir, committish)
	cmd.Dir = repoPath
	_, err = runCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to construct temp worktree with command %v: %w", cmd, err)
	}
	return &TempWorktree{
		repoPath:     repoPath,
		worktreePath: tmpDir,
	}, nil
}

// Root returns the path to the root directory of the worktree.
func (w *TempWorktree) Root() string {
	return w.worktreePath
}

// Close deletes the worktree from disk and unregisters it from git.
//
// If a worktree is not closed, `git worktree list` will show the associated
// directory.
// If a worktree is deleted out-of-band of `git worktree remove`, `git worktree
// list` will still show it as registered but prunable; the list can be cleaned
// up with `git worktree prune`.
func (w *TempWorktree) Close() error {
	cmd := exec.Command("git", "worktree", "remove", w.worktreePath)
	cmd.Dir = w.repoPath
	_, err := runCommand(cmd)
	if err != nil {
		err = fmt.Errorf("failed to delete temp git worktree at %q: %w", w.worktreePath, err)
		return err
	}
	return nil
}
