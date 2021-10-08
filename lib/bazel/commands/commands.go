package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enfabrica/enkit/lib/bazel"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/git"

	"github.com/spf13/cobra"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags
}

func New(base *client.BaseFlags) *Root {
	root := NewRoot(base)

	root.AddCommand(NewAffectedTargets(root).Command)

	return root
}

func NewRoot(base *client.BaseFlags) *Root {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "bazel",
			Short:         "Perform bazel helper actions",
			SilenceUsage:  true,
			SilenceErrors: true,
			Long:          `bazel - performs helper bazel operations`,
		},
		BaseFlags: base,
	}
	return rc
}

type AffectedTargets struct {
	*cobra.Command
	root *Root

	Start    string
	End      string
	Universe []string
}

func NewAffectedTargets(root *Root) *AffectedTargets {
	command := &AffectedTargets{
		Command: &cobra.Command{
			Use:     "affected-targets",
			Short:   "Operations involving changed bazel targets between two source revision points",
			Aliases: []string{"at"},
		},
		root: root,
	}

	command.PersistentFlags().StringVarP(&command.Start, "start", "s", "HEAD", "Git committish of 'before' revision")
	command.PersistentFlags().StringVarP(&command.End, "end", "e", "", "Git committish of 'end' revision, or empty for current dir with uncomitted changes")
	command.PersistentFlags().StringSliceVarP(&command.Universe, "universe", "u", []string{"//..."}, "Target universe in which to search for dependencies")

	command.AddCommand(NewAffectedTargetsList(command).Command)

	return command
}

type AffectedTargetsList struct {
	*cobra.Command
	parent *AffectedTargets
}

func NewAffectedTargetsList(parent *AffectedTargets) *AffectedTargetsList {
	command := &AffectedTargetsList{
		Command: &cobra.Command{
			Use:     "list",
			Short:   "List affected targets between two source revision points",
			Aliases: []string{"ls"},
			Example: `  $ enkit bazel affected-targets list
        List affected targets between the last commit and current uncommitted changes.

  $ enkit bazel affected-targets list --start=HEAD~1 --end=HEAD
        List affected targets in the most recent commit.`,
		},
		parent: parent,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *AffectedTargetsList) Run(cmd *cobra.Command, args []string) error {
	// TODO(scott): Determine how the workspace root is found
	gitRoot, err := findGitRoot()
	if err != nil {
		return fmt.Errorf("can't find git repo root: %w", err)
	}

	// Create temporary worktrees in which to execute bazel commands.
	// If the end commit is not provided, use the current git directory as the end
	// worktree, which will include uncommitted local changes.
	startTree, err := git.NewTempWorktree(gitRoot, c.parent.Start)
	if err != nil {
		return fmt.Errorf("can't generate worktree for committish %q: %w", c.parent.Start, err)
	}
	defer startTree.Close()

	endTreePath := gitRoot
	if c.parent.End != "" {
		endTree, err := git.NewTempWorktree(gitRoot, c.parent.End)
		if err != nil {
			return fmt.Errorf("can't generate worktree for committish %q: %w", c.parent.End, err)
		}
		defer endTree.Close()
		endTreePath = endTree.Root()
	}

	// Open the bazel workspaces, using a well-known output_base. Since the
	// temporary worktrees created above will have a different path on every
	// invocation, by default bazel will create a new cache directory for them,
	// re-download all dependencies, etc. which is both slow and will eventually
	// fill up the disk. Reusing an output base location will ensure that at most
	// only two are created for the purposes of this subcommand.
	startOutputBase, err := cacheDir("affected_targets/start")
	if err != nil {
		return fmt.Errorf("failed to create output_base: %w", err)
	}
	endOutputBase, err := cacheDir("affected_targets/end")
	if err != nil {
		return fmt.Errorf("failed to create output_base: %w", err)
	}
	startWorkspace, err := bazel.OpenWorkspace(startTree.Root(), bazel.WithOutputBase(startOutputBase))
	if err != nil {
		return fmt.Errorf("failed to open bazel workspace for committish %q: %w", c.parent.Start, err)
	}
	endWorkspace, err := bazel.OpenWorkspace(endTreePath, bazel.WithOutputBase(endOutputBase))
	if err != nil {
		return fmt.Errorf("failed to open bazel workspace: %w", err)
	}

	// Get all target info for both VCS time points.
	targets, err := startWorkspace.Query("deps(//...)", bazel.WithKeepGoing(), bazel.WithUnorderedOutput())
	if err != nil {
		return fmt.Errorf("failed to query deps for start point: %w", err)
	}
	// TODO(scott): Replace with logging
	fmt.Fprintf(os.Stderr, "Processed %d targets at start point\n", len(targets))

	targets, err = endWorkspace.Query("deps(//...)", bazel.WithKeepGoing(), bazel.WithUnorderedOutput())
	if err != nil {
		return fmt.Errorf("failed to query deps for end point: %w", err)
	}
	// TODO(scott): Replace with logging
	fmt.Fprintf(os.Stderr, "Processed %d targets at end point\n", len(targets))

	return fmt.Errorf("not yet implemented")
}

func cacheDir(subDir string) (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "enkit/bazel", subDir), nil
}

func findGitRoot() (string, error) {
	bazelWorkspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if bazelWorkspace != "" {
		return bazelWorkspace, nil
	}
	return git.RootFromPwd()
}
