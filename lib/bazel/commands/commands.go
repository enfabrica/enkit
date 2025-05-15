package commands

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ppb "github.com/enfabrica/enkit/enkit/proto"
	"github.com/enfabrica/enkit/lib/bazel"
	"github.com/enfabrica/enkit/lib/bes"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/git"
	"github.com/enfabrica/enkit/lib/logger"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/protobuf/encoding/prototext"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	BuildBuddyApiKey string
	BuildBuddyUrl    string
}

func New(base *client.BaseFlags) *Root {
	root := NewRoot(base)

	root.AddCommand(NewAffectedTargets(root).Command)
	root.AddCommand(NewInvocations(root).Command)

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

	rc.PersistentFlags().StringVar(&rc.BuildBuddyApiKey, "buildbuddy-api-key", "", "build buddy api key used to bypass oauth2")
	rc.PersistentFlags().StringVar(&rc.BuildBuddyUrl, "buildbuddy-url", "", "build buddy url instance")

	return rc
}

func (r *Root) BuildBuddyClient() (*bes.BuildBuddyClient, error) {
	buddyUrl, err := url.Parse(r.BuildBuddyUrl)
	if err != nil {
		return nil, fmt.Errorf("failed parsing buildbuddy url: %w", err)
	}
	bc, err := bes.NewBuildBuddyClient(buddyUrl, r.BaseFlags, r.BuildBuddyApiKey)
	if err != nil {
		return nil, fmt.Errorf("failed generating new buildbuddy client: %w", err)
	}
	return bc, nil
}

type Invocations struct {
	*cobra.Command
	root *Root

	InvID string
}

func NewInvocations(root *Root) *Invocations {
	command := &Invocations{
		Command: &cobra.Command{
			Use:     "invocations",
			Short:   "Operations querying targets on a particular bazel invocation",
			Aliases: []string{"inv", "invs"},
		},
		root: root,
	}

	command.PersistentFlags().StringVarP(&command.InvID, "invocation-id", "i", "", "Invocation ID of build to use")
	command.MarkFlagRequired("invocation-id")

	command.AddCommand(NewInvTargetsList(command).Command)

	return command
}

type InvTargetsList struct {
	*cobra.Command
	root   *Root
	parent *Invocations

	StatusFilter []string
}

func NewInvTargetsList(parent *Invocations) *InvTargetsList {
	command := &InvTargetsList{
		Command: &cobra.Command{
			Use:     "list-targets",
			Short:   "List targets in invocation",
			Aliases: []string{"lt"},
		},
		root:   parent.root,
		parent: parent,

		StatusFilter: nil,
	}
	command.Command.PreRunE = command.Check
	command.Command.RunE = command.Run
	command.Flags().StringSliceVar(&command.StatusFilter, "status-filter", nil, "If set, filter targets by criteria")

	return command
}

func (c *InvTargetsList) Check(cmd *cobra.Command, args []string) error {
	// Ensure invocation ID is set
	if c.parent.InvID == "" {
		return fmt.Errorf("--invocation-id must be set")
	}
	if c.StatusFilter != nil {
		for _, s := range c.StatusFilter {
			if _, ok := bespb.TestStatus_value[strings.ToUpper(s)]; !ok {
				return fmt.Errorf("invalid status %q; valid statuses: [%s]", s, strings.Join(validStatuses(), " "))
			}
		}
	}
	return nil
}

func (c *InvTargetsList) Run(cmd *cobra.Command, args []string) error {
	// Fetch invocation data
	bb, err := c.root.BuildBuddyClient()
	if err != nil {
		return fmt.Errorf("failed to connect to BuildBuddy: %w", err)
	}
	events, err := bb.GetBuildEvents(cmd.Context(), c.parent.InvID)
	if err != nil {
		return fmt.Errorf("failed to get invocation with ID %q: %w", c.parent.InvID, err)
	}
	statuses, err := invocationStatusFromBuildEvents(events)
	if err != nil {
		return fmt.Errorf("failed to parse status of all targets: %w", err)
	}
	statuses = statuses.Filter(c.StatusFilter...)

	if term.IsTerminal(int(os.Stdout.Fd())) {
		if err := statuses.PrettyPrint(os.Stdout); err != nil {
			return fmt.Errorf("failed to print target statuses: %w", err)
		}
	} else {
		if err := statuses.Print(os.Stdout); err != nil {
			return fmt.Errorf("failed to print target statuses: %w", err)
		}
	}
	return nil
}

type AffectedTargets struct {
	*cobra.Command
	root *Root

	Start           string
	End             string
	RepoRoot        string
	PresubmitConfig string
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
	command.PersistentFlags().StringVar(&command.PresubmitConfig, "presubmit_config", "", "Path to presubmit configuration to read target filtering options")

	command.AddCommand(NewAffectedTargetsList(command).Command)

	return command
}

type AffectedTargetsList struct {
	*cobra.Command
	root   *Root
	parent *AffectedTargets

	AffectedTargetsFile string
	AffectedTestsFile   string
	Parallel            bool

	bazel.GetModeOptions
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

	command.Flags().BoolVar(&command.Parallel, "parallel", true, "If set, the bazel query is run in parallel")
	command.Flags().StringVar(&command.AffectedTargetsFile, "affected_targets_file", "", "If set, the list of affected targets will be dumped to this file path")
	command.Flags().StringVar(&command.AffectedTestsFile, "affected_tests_file", "", "If set, the list of affected tests will be dumped to this file path")
	command.Flags().StringVar(&command.Start.OutputBase, "start_output_base", "", "If set, the directory to use as start output base")
	command.Flags().StringVar(&command.Start.WorkspaceLog, "start_workspace_log", "", "If set, previously generated workspace log file will be used")
	command.Flags().StringVar(&command.End.OutputBase, "end_output_base", "", "If set, the directory to use as end output base")
	command.Flags().StringVar(&command.End.WorkspaceLog, "end_workspace_log", "", "If set, previously generated workspace log file will be used")
	command.Flags().StringVar(&command.Query, "query", "deps(//...)",
		"The query to use to find the targets. Only the default query has been tested, "+
			"not all queries will work correctly, make sure to test your changes carefully")
	command.Flags().StringArrayVar(&command.ExtraStartup, "extra_startup", nil, "Extra startup flags appended to the bazel command line")

	return command
}

// setCurrentOutputBaseAsDefault takes the output base of the bazel workspace
// specified in gitRoot and sets it as the default for Start and End OutputBase,
// assuming one was not specified from the command line already.
func setCurrentOutputBaseAsDefault(gitRoot string, options *bazel.GetModeOptions, log logger.Logger) error {
	if options.Start.OutputBase != "" && options.End.OutputBase != "" {
		return nil
	}

	ws, err := bazel.OpenWorkspace(gitRoot, bazel.WithExtraStartupFlags(options.ExtraStartup...), bazel.WithLogging(log))
	if err != nil {
		return err
	}

	obase, err := ws.OutputBaseDir()
	if err != nil {
		return err
	}

	if options.Start.OutputBase == "" {
		options.Start.OutputBase = obase
	}

	if options.End.OutputBase == "" {
		options.End.OutputBase = obase
	}

	return nil
}

func (c *AffectedTargetsList) Run(cmd *cobra.Command, args []string) error {
	config := defaultConfig()
	if c.parent.PresubmitConfig != "" {
		var err error
		config, err = readConfig(c.parent.PresubmitConfig)
		if err != nil {
			return err
		}
	}

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
	c.Start.RepoPath = filepath.Clean(filepath.Join(startTree.Root(), gitToBazelPath))
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
	c.End.RepoPath = filepath.Clean(filepath.Join(endTreePath, gitToBazelPath))

	mode := bazel.ParallelQuery
	if !c.Parallel {
		if err := setCurrentOutputBaseAsDefault(gitRoot, &c.GetModeOptions, c.root.Log); err != nil {
			c.root.Log.Warnf("Could not compute output_base of workspace %s: %s", gitRoot, err)
		}

		mode = bazel.SerialQuery
	}

	rules, tests, err := bazel.GetAffectedTargets(config, mode, c.GetModeOptions, c.root.BaseFlags.Log)
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

func readConfig(configPath string) (*ppb.PresubmitConfig, error) {
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %w", configPath, err)
	}
	var config ppb.PresubmitConfig
	if err := prototext.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %q: %w", configPath, err)
	}
	return &config, nil
}

func defaultConfig() *ppb.PresubmitConfig {
	return &ppb.PresubmitConfig{
		IncludePatterns: []string{"//..."},
	}
}

// bazelGitRoot returns:
//   - The git worktree root path for the current bazel workspace. This is
//     either:
//   - the workspace from which `bazel run` was executed, if running under
//     `bazel run`
//   - the workspace containing the current working directory otherwise
//   - A relative path between the git worktree root and the bazel workspace root
//     for the aforementioned bazel workspace. If the bazel workspace root and
//     git worktree root are the same, this is the path `.`.
//   - An error of detection of the following fails:
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
