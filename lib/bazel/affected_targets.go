package bazel

import (
	"fmt"
	"os"
	"path/filepath"
	"io/ioutil"
)

func GetAffectedTargets(start string, end string) ([]string, error) {
	// Open the bazel workspaces, using a temporary output_base. Since the
	// temporary worktrees created above will have a different path on every
	// invocation, by default bazel will create a new cache directory for them,
	// re-download all dependencies, etc. which is both slow and will eventually
	// fill up the disk. Using a temporary output_base which gets deleted each
	// time avoids this problem, at the cost of the startup/redownload on
	// repeated invocations with the same source points.
	//
	// This temporary directory needs to be in the user's $HOME directory to
	// avoid filling up /tmp in the dev container.
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user's cache dir: %w", err)
	}
	cacheDir = filepath.Join(cacheDir, "enkit", "bazel")
	startOutputBase, err := ioutil.TempDir(cacheDir, "output_base_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary output_base: %w", err)
	}
	defer os.RemoveAll(startOutputBase)

	endOutputBase, err := ioutil.TempDir(cacheDir, "output_base_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary output_base: %w", err)
	}
	defer os.RemoveAll(endOutputBase)

	// Joining the new worktree roots to the relative path portion handles the
	// case where bazel workspaces are not in the top directory of the git
	// worktree.
	startWorkspace, err := OpenWorkspace(
		start,
		WithOutputBase(startOutputBase),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}
	endWorkspace, err := OpenWorkspace(
		end,
		WithOutputBase(endOutputBase),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}

	workspaceLogStart, err := WithTempWorkspaceRulesLog()
	if err != nil {
		return nil, fmt.Errorf("start workspace: %w", err)
	}
	workspaceLogEnd, err := WithTempWorkspaceRulesLog()
	if err != nil {
		return nil, fmt.Errorf("end workspace: %w", err)
	}

	// Get all target info for both VCS time points.
	startResults, err := startWorkspace.Query("deps(//...)", WithKeepGoing(), WithUnorderedOutput(), workspaceLogStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query deps for start point: %w", err)
	}

	endResults, err := endWorkspace.Query("deps(//...)", WithKeepGoing(), WithUnorderedOutput(), workspaceLogEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query deps for end point: %w", err)
	}

	// TODO(scott): Implement diffing of returned targets
	startResults = startResults
	endResults = endResults

	return nil, fmt.Errorf("not implemented")
}
