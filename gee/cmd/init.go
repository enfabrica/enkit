package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/enfabrica/enkit/gee/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Flags:
var (
	flagAll bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "Initialize a local gee repository.",
	Long:    `Creates a new gee workspace in your ~/gee directory.`,
	Example: `gee init --select internal`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("init called")
		lib.InstallTools()
		if !lib.CheckSsh() {
			lib.SshEnroll()
		}
		lib.Runner().RunGh(lib.Cmd{}, "config", "set", "git_protocol", "ssh")
		lib.CheckGhAuth()
		repoConfig := lib.NewRepoConfig(cmd.Flags())

		lib.Logger().Infof("Initializing %q for %s/%s", repoConfig.GetRepoDir(),
			repoConfig.Upstream, repoConfig.Repository)
		lib.Runner().Run(lib.Cmd{}, "/usr/bin/mkdir", "-p", repoConfig.GetRepoDir())

		origin_url := fmt.Sprintf("%s:%s/%s.git",
			viper.GetString("git_ssh_username"),
			viper.GetString("ghuser"),
			repoConfig.Repository)
		upstream_url := fmt.Sprintf("%s:%s/%s.git",
			viper.GetString("git_ssh_username"),
			repoConfig.Upstream,
			repoConfig.Repository)
		if !lib.RepoExists(viper.GetString("ghuser"), repoConfig.Repository) {
			lib.Runner().RunGh(lib.Cmd{},
				"repo", "fork", "--clone=false",
				fmt.Sprintf("%s/%s", repoConfig.Upstream, repoConfig.Repository))
		}
		// Assume main branch name is "main" until proven otherwise:
		default_main_path := filepath.Join(repoConfig.GetRepoDir(), "main")
		lib.Runner().RunGit(lib.Cmd{},
			"clone",
			fmt.Sprintf("--depth=%d", viper.GetInt("clone_depth")),
			"--no-single-branch",
			origin_url,
			default_main_path)
		lib.Runner().ChDir(default_main_path)
		lib.Runner().RunGit(lib.Cmd{},
			"remote", "add", "upstream", upstream_url)
		lib.Runner().RunGit(lib.Cmd{}, "fetch", "upstream")
		lib.Runner().RunGit(lib.Cmd{}, "remote", "-v")

		// Fix the name of the main branch
		main, err := repoConfig.GetMainBranchNameFromGitHub()
		if err != nil {
			lib.Logger().Fatalf("Could not determine main branch name: %q", err)
		}
		if main != "main" {
			lib.Runner().ChDir(repoConfig.GetRepoDir())
			err = os.Rename(default_main_path,
				repoConfig.GetRepoDir()+"/"+main)
			lib.Runner().ChDir(repoConfig.GetRepoDir() + "/" + main)
		}
		lib.Logger().Infof("Created %s/%s", repoConfig.GetRepoDir(), main)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&flagAll, "all", "a", false,
		"Initialize all repos in the configuration file.")
}
