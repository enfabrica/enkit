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
	RepoRoot string
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
	command.PersistentFlags().StringVarP(&command.RepoRoot, "repo_root", "r", "", "Path to the git repository root; autodetected from $PWD if unset")
	command.PersistentFlags().StringSliceVarP(&command.Universe, "universe", "u", []string{"//..."}, "Target universe in which to search for dependencies")

	command.AddCommand(NewAffectedTargetsList(command).Command)

	return command
}

type AffectedTargetsList struct {
	*cobra.Command
	root   *Root
	parent *AffectedTargets

	AffectedTargetsFile string
	AffectedTestsFile   string
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
		root:   parent.root,
		parent: parent,
	}
	command.Command.RunE = command.Run
	command.Flags().StringVar(&command.AffectedTargetsFile, "affected_targets_file", "", "If set, the list of affected targets will be dumped to this file path")
	command.Flags().StringVar(&command.AffectedTestsFile, "affected_tests_file", "", "If set, the list of affected tests will be dumped to this file path")
	return command
}

func (c *AffectedTargetsList) Run(cmd *cobra.Command, args []string) error {
	startDir := c.parent.RepoRoot
	var err error
	if startDir == "" {
		startDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to detect working dir: %w", err)
		}
	}
	gitRoot, gitToBazelPath, err := bazelGitRoot(startDir)
	if err != nil {
		return fmt.Errorf("can't find git repo root: %w", err)
	}

	// Create temporary worktrees in which to execute bazel commands.
	// If the end commit is not provided, use the current git directory as the end
	// worktree, which will include uncommitted local changes.
	c.root.BaseFlags.Log.Infof("Checking out %q to tempdir...", c.parent.Start)
	startTree, err := git.NewTempWorktree(gitRoot, c.parent.Start)
	if err != nil {
		return fmt.Errorf("can't generate worktree for committish %q: %w", c.parent.Start, err)
	}
	defer startTree.Close()
	startWS := filepath.Clean(filepath.Join(startTree.Root(), gitToBazelPath))
	c.root.BaseFlags.Log.Infof("Checked out %q to %q", c.parent.Start, startTree.Root())

	endTreePath := gitRoot
	if c.parent.End != "" {
		c.root.BaseFlags.Log.Infof("Checking out %q to tempdir...", c.parent.End)
		endTree, err := git.NewTempWorktree(gitRoot, c.parent.End)
		if err != nil {
			return fmt.Errorf("can't generate worktree for committish %q: %w", c.parent.End, err)
		}
		defer endTree.Close()
		endTreePath = endTree.Root()
	} else {
		c.root.BaseFlags.Log.Infof("Using %d as ending working directory", endTreePath)
	}
	c.root.BaseFlags.Log.Infof("Checked out %q to %q", c.parent.End, endTreePath)
	endWS := filepath.Clean(filepath.Join(endTreePath, gitToBazelPath))

	rules, tests, err := bazel.GetAffectedTargets(startWS, endWS, c.root.BaseFlags.Log)
	if err != nil {
		return fmt.Errorf("failed to calculate affected targets: %w", err)
	}

	if c.AffectedTargetsFile != "" {
		err = writeTargets(rules, c.AffectedTargetsFile)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Affected targets:")
		for _, t := range rules {
			fmt.Println(t.Name())
		}
		fmt.Printf("\n")
	}

	if c.AffectedTestsFile != "" {
		err = writeTargets(tests, c.AffectedTestsFile)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Affected tests:")
		for _, t := range tests {
			fmt.Println(t.Name())
		}
	}
	return nil
}

func writeTargets(targets []*bazel.Target, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create target file %q: %w", path, err)
	}
	defer f.Close()
	for _, t := range targets {
		fmt.Fprintf(f, "%s\n", t.Name())
	}
	return nil
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
func bazelGitRoot(dir string) (string, string, error) {
	bazelRoot, err := bazel.FindRoot(dir)
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
