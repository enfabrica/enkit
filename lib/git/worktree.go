package git

import (
	"fmt"
	"io/ioutil"
	"os/exec"

	"github.com/golang/glog"
)

type TempWorktree struct {
	repoPath string
	worktreePath string
}

var runCommand = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

func NewTempWorktree(repoPath string, committish string) (*TempWorktree, error) {
	tmpDir, err := ioutil.TempDir("", "git_temp_worktree_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	cmd := exec.Command("git", "worktree", "add", tmpDir, committish)
	cmd.Dir = repoPath
	_, err = runCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to construct temp worktree: %v", err)
	}
	return &TempWorktree{
		repoPath: repoPath,
		worktreePath: tmpDir,
	}, nil
}

func (w *TempWorktree) Root() string {
	return w.worktreePath
}

func (w *TempWorktree) Close() error {
	cmd := exec.Command("git", "worktree", "remove", w.worktreePath)
	cmd.Dir = w.repoPath
	_, err := runCommand(cmd)
	if err != nil {
		err = fmt.Errorf("failed to delete temp git worktree at %q: %v", w.worktreePath, err)
		glog.Warning(err)
		return err
	}
	return nil
}