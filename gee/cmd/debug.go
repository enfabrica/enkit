package cmd

import (
	"fmt"
	"github.com/enfabrica/enkit/gee/lib"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoConfig := lib.NewRepoConfig(cmd.Flags())
		l := lib.Logger()
		l.Info(fmt.Sprintf("MaxColors=%d", l.GetMaxColors()))
		l.Info(fmt.Sprintf("Columns=%d", l.GetColumns()))
		l.Debug("debug")
		l.Info("info")
		l.Banner("banner!", "This is a banner.")
		l.Info(
			"settings from "+viper.ConfigFileUsed(),
			"upstream: "+repoConfig.Upstream,
			"repository: "+repoConfig.Repository,
			viper.GetString("git_path"))
		lib.Runner().Run(lib.Cmd{}, "/usr/bin/pwd")
		lib.Runner().RunGit(lib.Cmd{}, "version")
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// debugCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// debugCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}