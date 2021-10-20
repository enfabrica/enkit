package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
)

// FindRoot returns a path to the root of the current Git repository in which
// `dir` resides, or an error if `dir` is not inside a Git repository.
func FindRoot(dir string) (string, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
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
