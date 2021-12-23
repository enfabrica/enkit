package cmd

import (
	"github.com/enfabrica/enkit/gee/lib"
	"github.com/spf13/cobra"
)

var (
	flagMessage string
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:     "commit [-m message]",
	Aliases: []string{"c", "co"},
	Short:   "Commit all changes in this branch.",
	Long: `Commits all outstanding changes (additions, changes, deletions) to your
repository, and then backs up your work to your private github repository
("origin").`,
	Example: `gee commit -m "Fixed BUILD file."`,
	Run: func(cmd *cobra.Command, args []string) {
		lib.CheckInGeeRepo()
		repoConfig := lib.NewRepoConfig(cmd.Flags())
		main_branch := repoConfig.GetMainBranch()
		current_branch := lib.GetCurrentBranch()
		if current_branch == main_branch {
			lib.Logger().Info(
				"gee's workflow doesn't let you make commits to the master branch.",
				"You should move your changes to another branch.  For example:",
				"  git add -a; git stash; gcd -b new1; git stash apply")
			lib.Logger().Fatalf("Commit to %q branch denied.", main_branch)
		}

		lib.Runner().RunGit(lib.Cmd{}, "add", "--all")
		commit := []string{"commit", "--all"}
		if flagMessage != "" {
			commit = append(commit, "-m", flagMessage)
		}
		commit = append(commit, args...)
		lib.Runner().RunGit(lib.Cmd{}, commit...)
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)

	commitCmd.Flags().StringVarP(&flagMessage, "message", "m", "",
		"Specify a commit message.")
}
