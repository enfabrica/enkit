package bazel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GetAffectedTargets(start string, end string) ( /* changedRules */ []*Target /* changedTests */, []*Target, error) {
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
		return nil, nil, fmt.Errorf("failed to get user's cache dir: %w", err)
	}
	cacheDir = filepath.Join(cacheDir, "enkit", "bazel")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make root cache dir: %w", err)
	}
	startOutputBase, err := ioutil.TempDir(cacheDir, "output_base_*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary output_base: %w", err)
	}
	defer os.RemoveAll(startOutputBase)

	endOutputBase, err := ioutil.TempDir(cacheDir, "output_base_*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary output_base: %w", err)
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
		return nil, nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}
	endWorkspace, err := OpenWorkspace(
		end,
		WithOutputBase(endOutputBase),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}

	workspaceLogStart, err := WithTempWorkspaceRulesLog()
	if err != nil {
		return nil, nil, fmt.Errorf("start workspace: %w", err)
	}
	workspaceLogEnd, err := WithTempWorkspaceRulesLog()
	if err != nil {
		return nil, nil, fmt.Errorf("end workspace: %w", err)
	}

	// Get all target info for both VCS time points.
	startResults, err := startWorkspace.Query("deps(//...)", WithKeepGoing(), WithUnorderedOutput(), workspaceLogStart)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query deps for start point: %w", err)
	}

	endResults, err := endWorkspace.Query("deps(//...)", WithKeepGoing(), WithUnorderedOutput(), workspaceLogEnd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query deps for end point: %w", err)
	}

	diff, err := calculateAffected(startResults, endResults)
	if err != nil {
		return nil, nil, err
	}

	var changedRules []*Target
	var changedTests []*Target
	for _, targetName := range diff {
		target := endResults.Targets[targetName]
		if target.ruleType() == "" {
			continue
		}
		changedRules = append(changedRules, target)
		if strings.HasSuffix(target.ruleType(), "_test") {
			changedTests = append(changedTests, target)
		}
	}
	sort.Slice(changedRules, func(i, j int) bool { return changedRules[i].Name() > changedRules[j].Name() })
	sort.Slice(changedTests, func(i, j int) bool { return changedTests[i].Name() > changedTests[j].Name() })

	return changedRules, changedTests, nil
}

func calculateAffected(startResults, endResults *QueryResult) ([]string, error) {
	startHashes, err := startResults.TargetHashes()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target hashes for start point: %w", err)
	}
	endHashes, err := endResults.TargetHashes()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target hashes for end point: %w", err)
	}
	diff := endHashes.Diff(startHashes)
	sort.Strings(diff)
	return diff, nil
}
