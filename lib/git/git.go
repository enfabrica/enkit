package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
)

// RootFromPwd returns a path to the root of the current Git repository in
// which the present working directory resides, or an error if PWD is not
// inside a Git repository.
func RootFromPwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get PWD: %w", err)
	}
	repo, err := git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open git repo: %w", err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	return tree.Filesystem.Root(), nil
}
