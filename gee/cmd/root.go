package cmd

import (
	"fmt"
	"github.com/enfabrica/enkit/gee/lib"
	"github.com/spf13/cobra"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Other than cfgFile, all other rootCmd flag options are stored in viper.
var cfgFile string

// rootCmd represents the base command when called without any subcmd
var rootCmd = &cobra.Command{
	Use:   "gee",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child cmd to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.gee.yaml)")
	rootCmd.PersistentFlags().String("upstream", "",
		"The github account associated with the upstream repository (ie, enfabrica).")
	rootCmd.PersistentFlags().String("repository", "",
		"The github repository (ie, enkit).")
	rootCmd.PersistentFlags().String("select", "",
		"Which \"repos\"  entry from the configuration file to use.")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
//
// Example config file:
//
//     ghuser: foobar
//     repos:
//       internal:
//         upstream: enfabrica
//         repo: internal
//       enkit:
//         upstream: enfabrica
//         repo: enkit
//     default_repo: internal
func initConfig() {
	viper.SetDefault("upstream", "")
	viper.SetDefault("repository", "")
	viper.SetDefault("git_path", "/usr/bin/git")
	viper.SetDefault("gh_path", "/usr/bin/gh")
	viper.SetDefault("jq_path", "/usr/bin/jq")
	viper.SetDefault("astore_path", "test/gee-beta")
	viper.SetDefault("ghuser", "")
	viper.SetDefault("gh_ssh_keyfile", "~/.ssh/gee_github_ed25519")
	viper.SetDefault("clone_depth", 3)
	viper.SetDefault("verbosity", 0)
	viper.SetDefault("editor", "/usr/bin/vim")
	viper.SetDefault("pager", "/usr/bin/less")
	viper.SetDefault("default_repo", "")

	viper.SetConfigType("yaml")
	if cfgFile != "" {
		// Use config file from the flag.
		fmt.Println("foo", cfgFile)
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".gee" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gee")
	}

	viper.SetEnvPrefix("GEE_")
	viper.AutomaticEnv() // read in environment variables that match
	viper.BindEnv("ghuser", "GEE_GHUSER", "GHUSER")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Missing .gee configuration file.")
			// TODO(jonathan): create a skeleton file?
		} else {
			fmt.Println("Error parsing config file: ", err)
			os.Exit(1)
		}
	}

	// Override some flags:
	// Use --select to choose a non-default repository to use.
	flags := rootCmd.PersistentFlags()
	selectRepo, _ := flags.GetString("select")
	if selectRepo == "" {
		selectRepo = viper.GetString("default_repo")
	}
	if selectRepo != "" {
		if !viper.IsSet("repos." + selectRepo) {
			fmt.Println("Error: repos." + selectRepo + " is not in your config file.")
			os.Exit(1)
		}
		if upstream := viper.Get("repos." + selectRepo + ".upstream"); upstream != "" {
			viper.Set("upstream", upstream)
		}
		if repository := viper.Get("repos." + selectRepo + ".repository"); repository != "" {
			viper.Set("repository", repository)
		}
	} // selectRepo
	if upstream, err := flags.GetString("upstream"); (err != nil) && (upstream != "") {
		viper.Set("upstream", upstream)
	}
	if repository, err := flags.GetString("repository"); (err != nil) && (repository != "") {
		viper.Set("repository", repository)
	}
	if viper.GetString("upstream") == "" {
		lib.GetLogger().Fatal(`Error: "upstream" is not configured.`)
	}
	if viper.GetString("repository") == "" {
		lib.GetLogger().Fatal(`Error: "repository" is not configured.`)
	}
}
