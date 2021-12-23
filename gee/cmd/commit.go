package cmd

import (
	"fmt"

	"github.com/enfabrica/enkit/gee/lib"
	"github.com/spf13/cobra"
)

var (
	flagAll     bool
	flagMessage string
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("commit called")
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

		lib.Runner().RunGit("add", "--all")
		commit := []string{"commit"}
		if flagAll {
			commit = append(commit, "--all")
		}
		if flagMessage != "" {
			commit = append(commit, "-m", flagMessage)
		}
		commit = append(commit, args...)
		lib.Runner().RunGit(commit...)

	},
}

func init() {
	rootCmd.AddCommand(commitCmd)

	commitCmd.Flags().BoolVarP(&flagAll, "all", "a", false,
		"Automatically stage files that have been modified or deleted.")
	commitCmd.Flags().StringVarP(&flagMessage, "message", "m", "",
		"Specify a commit message.")
}
