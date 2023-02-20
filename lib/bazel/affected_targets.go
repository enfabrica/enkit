package bazel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ppb "github.com/enfabrica/enkit/enkit/proto"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
)

type SourceOptions struct {
	// Path of the git repository containing the start/end code.
	RepoPath string
	// Path of the --output_base to use with bazel.
	// If empty, a default path will be used depending on the GetMode used.
	OutputBase string
}

type GetModeOptions struct {
	// The starting point to compute the diff.
	Start SourceOptions
	// The end point to compute the diff.
	End SourceOptions

	// Bazel query to use to find all the targets. Must be set.
	// A good default is "deps(//...)"
	Query string

	// Extra startup flags to pass to bazel.
	// Those flags are appended after other common flags (but before
        // subcommand flags), so should override any global flag.
	ExtraStartup []string
}

// The result of running a GetMode function below.
type GetResult struct {
	StartQueryResult *QueryResult
	StartQueryError  error
	EndQueryResult   *QueryResult
	EndQueryError    error
}

// GetMode is a function to run bazel queries.
//
// As paramters, it takes a temporary directory to use for the work,
// and a GetModeOptions object.
//
// There are currently two implementations of GetMode: ParallelQuery, and SerialQuery.
type GetMode func(options GetModeOptions, log logger.Logger) (*GetResult, error)

func GetAffectedTargets(config *ppb.PresubmitConfig, mode GetMode, opts GetModeOptions, log logger.Logger) ( /* changedRules */ []*Target /* changedTests */, []*Target, error) {
	includePatterns, err := NewPatternSet(config.GetIncludePatterns())
	if err != nil {
		return nil, nil, err
	}
	excludePatterns, err := NewPatternSet(config.GetExcludePatterns())
	if err != nil {
		return nil, nil, err
	}
	excludeTags := config.GetExcludeTags()
        includeTags := config.GetIncludeTags()

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
	if opts.Start.OutputBase == "" {
		opts.Start.OutputBase, err = ioutil.TempDir(cacheDir, "output_base_*")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temporary output_base: %w", err)
		}
		defer os.RemoveAll(opts.Start.OutputBase)
	}

	if opts.End.OutputBase == "" {
		opts.End.OutputBase, err = ioutil.TempDir(cacheDir, "output_base_*")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create temporary output_base: %w", err)
		}
		defer os.RemoveAll(opts.End.OutputBase)
	}

	result, errs := mode(opts, log)

	if errs != nil {
		if result.StartQueryError != nil && result.EndQueryError == nil {
			// We are calculating targets over a change that fixes the build graph
			// (broken before, working after). In a presubmit context, we want this
			// step to succeed, but there is no sensible list of targets that the
			// change affects since the build graph was broken in one stage.
			//
			// Pass here but emit a warning.
			log.Warnf("Got error at start point:\n%v\n", result.StartQueryError)
			log.Warnf("Broken build graph detected at start point; this change fixes the build graph, but no targets will be tested. This change must be tested manually.")
			return nil, nil, nil // No changed targets and no error
		}
		return nil, nil, errs
	}

	log.Infof("Calculating affected targets...")
	diff, err := calculateAffected(result.StartQueryResult, result.EndQueryResult)
	if err != nil {
		return nil, nil, err
	}
	log.Infof("Found %d affected targets", len(diff))

	log.Infof("Filtering targets...")
	var changedRules []*Target
	var changedTests []*Target

skipTarget:
	for _, targetName := range diff {
		target := result.EndQueryResult.Targets[targetName]
		if target.ruleType() == "" {
			log.Debugf("Filtering non-rule target %q", targetName)
			continue skipTarget
		}
		if !includePatterns.Contains(targetName) {
			log.Debugf("Filtering target not under include_patterns: %q", targetName)
			continue skipTarget
		}
		if excludePatterns.Contains(targetName) {
			log.Debugf("Filtering target under exclude_patterns: %q", targetName)
			continue skipTarget
		}
		for _, t := range excludeTags {
			if target.containsTag(t) {
				log.Debugf("Filtering target with excluded tag %q: %q", t, targetName)
				continue skipTarget
			}
		}

		if len(includeTags) > 0 {
			isIncluded := false
			for _, t := range includeTags {
				if target.containsTag(t) {
					isIncluded = true
					break
				}
			}
			if !isIncluded {
				log.Debugf("Filtering taget that does not include any of the included tags %q: %q", includeTags, targetName)
				continue skipTarget
			}
		}

		changedRules = append(changedRules, target)
		if strings.HasSuffix(target.ruleType(), "_test") {
			changedTests = append(changedTests, target)
		}
	}
	sort.Slice(changedRules, func(i, j int) bool { return changedRules[i].Name() > changedRules[j].Name() })
	sort.Slice(changedTests, func(i, j int) bool { return changedTests[i].Name() > changedTests[j].Name() })
	log.Infof("Found %d affected rule targets and %d affected tests", len(changedRules), len(changedTests))

	return changedRules, changedTests, nil
}

func SerialQuery(opt GetModeOptions, log logger.Logger) (*GetResult, error) {
	startWorkspace, err := OpenWorkspace(
		opt.Start.RepoPath,
		WithOutputBase(opt.Start.OutputBase),
		WithLogging(log),
		WithExtraStartupFlags(opt.ExtraStartup...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}
	endWorkspace, err := OpenWorkspace(
		opt.End.RepoPath,
		WithOutputBase(opt.End.OutputBase),
		WithLogging(log),
		WithExtraStartupFlags(opt.ExtraStartup...),
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

        var errs []error
	var result GetResult
	log.Infof("Querying dependency graph for 'before' workspace...")
	result.StartQueryResult, err = startWorkspace.Query(opt.Query, WithUnorderedOutput(), workspaceLogStart)
	if err != nil {
		result.StartQueryError = fmt.Errorf("failed to query deps for start point: %w", err)
		errs = append(errs, result.StartQueryError)
	} else {
		log.Infof("Queried info for %d targets from 'before' workspace", len(result.StartQueryResult.Targets))
	}

	log.Infof("Querying dependency graph for 'after' workspace...")
	result.EndQueryResult, err = endWorkspace.Query(opt.Query, WithUnorderedOutput(), workspaceLogEnd)
	if err != nil {
		result.EndQueryError = fmt.Errorf("failed to query deps for end point: %w", err)
		errs = append(errs, result.EndQueryError)
	} else {
		log.Infof("Queried info for %d targets from 'after' workspace", len(result.EndQueryResult.Targets))
	}

	return &result, multierror.New(errs)
}

func ParallelQuery(opt GetModeOptions, log logger.Logger) (*GetResult, error) {
	// BUG(INFRA-140) - By default, cargo will download packages to a well-known
	// directory under $HOME; this will mean that parallel bazel invocations could
	// race on this directory if they both fetch Cargo packages. Cargo respects
	// the $CARGO_HOME environment variable, so set it to something unique for
	// this invocation.
	cargoHomeStart, err := ioutil.TempDir("", "bazel_cargo_home_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmpdir for $CARGO_HOME: %w", err)
	}
	defer os.RemoveAll(cargoHomeStart)

	cargoHomeEnd, err := ioutil.TempDir("", "bazel_cargo_home_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmpdir for $CARGO_HOME: %w", err)
	}
	defer os.RemoveAll(cargoHomeEnd)

	// Joining the new worktree roots to the relative path portion handles the
	// case where bazel workspaces are not in the top directory of the git
	// worktree.
	startWorkspace, err := OpenWorkspace(
		opt.Start.RepoPath,
		WithOutputBase(opt.Start.OutputBase),
		WithLogging(log),
		WithExtraStartupFlags(opt.ExtraStartup...),
		WithExtraEnv("CARGO_HOME="+cargoHomeStart),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open bazel workspace: %w", err)
	}
	endWorkspace, err := OpenWorkspace(
		opt.End.RepoPath,
		WithOutputBase(opt.End.OutputBase),
		WithLogging(log),
		WithExtraStartupFlags(opt.ExtraStartup...),
		WithExtraEnv("CARGO_HOME="+cargoHomeEnd),
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
	var result GetResult
	errs := goroutine.WaitAll(
		func() error {
			log.Infof("Querying dependency graph for 'before' workspace...")
			var err error
			result.StartQueryResult, err = startWorkspace.Query(opt.Query, WithUnorderedOutput(), workspaceLogStart)
			if err != nil {
				result.StartQueryError = fmt.Errorf("failed to query deps for start point: %w", err)
				return result.StartQueryError
			}
			log.Infof("Queried info for %d targets from 'before' workspace", len(result.StartQueryResult.Targets))
			return nil
		},
		func() error {
			log.Infof("Querying dependency graph for 'after' workspace...")
			var err error
			result.EndQueryResult, err = endWorkspace.Query(opt.Query, WithUnorderedOutput(), workspaceLogEnd)
			if err != nil {
				result.EndQueryError = fmt.Errorf("failed to query deps for end point: %w", err)
				return result.EndQueryError
			}
			log.Infof("Queried info for %d targets from 'after' workspace", len(result.EndQueryResult.Targets))
			return nil
		},
	)
	return &result, errs
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
