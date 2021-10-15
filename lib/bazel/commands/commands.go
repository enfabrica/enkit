package commands

import (
	"fmt"
	"io/fs"
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
	gitRoot, gitToBazelPath, err := bazelGitRoot()
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
	startWS := filepath.Clean(filepath.Join(startTree.Root(), gitToBazelPath))

	endTreePath := gitRoot
	if c.parent.End != "" {
		endTree, err := git.NewTempWorktree(gitRoot, c.parent.End)
		if err != nil {
			return fmt.Errorf("can't generate worktree for committish %q: %w", c.parent.End, err)
		}
		//defer endTree.Close()
		endTreePath = endTree.Root()
	}
	endWS := filepath.Clean(filepath.Join(endTreePath, gitToBazelPath))

	targets, err := bazel.GetAffectedTargets(startWS, endWS)
	if err != nil {
		return fmt.Errorf("failed to calculate affected targets: %w", err)
	}

	targets = targets


	return fmt.Errorf("not yet implemented")
}

// bazelGitRoot returns:
// * The git worktree root path for the current bazel workspace. This is
//   either:
//   - the workspace from which `bazel run` was executed, if running under
//     `bazel run`
//   - the workspace containing the current working directory otherwise
// * A relative path between the git worktree root and the bazel workspace root
//   for the aforementioned bazel workspace. If the bazel workspace root and
//   git worktree root are the same, this is the path `.`.
// * An error of detection of the following fails:
//   - current working directory
//   - bazel workspace root
//   - git workspace root
func bazelGitRoot() (string, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to detect working dir: %w", err)
	}
	bazelRoot, err := bazel.FindRoot(wd)
	if err != nil {
		return "", "", err
	}
	root, err := git.FindRoot(bazelRoot)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, bazelRoot)
	if err != nil {
		return "", "", fmt.Errorf("can't calculate common path between %q and %q: %w", root, bazelRoot, err)
	}
	return root, rel, nil
}

func relativeWorkspace(gitRoot fs.FS, relativeWorkspace string) (fs.FS, error) {
	return nil, fmt.Errorf("not implemented")
}
