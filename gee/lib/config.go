package lib

import (
	"fmt"
	"os"
	"regexp"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type RepoConfig struct {
	Select     string
	Upstream   string
	Repository string
}

// NewRepoConfig determines upstream/repository settings based on the user's
// configuration, current working directory, and command line flags.  Most (but
// not all) commands will require this.
func NewRepoConfig(flags *flag.FlagSet) *RepoConfig {
	// Which repository is gee dealing with?
	// First, check for a --select flag on the command line:
	selectRepo, _ := flags.GetString("select")
	// Failing that, let's see if we're already in a gee directory:
	if selectRepo == "" {
		path, err := os.Getwd()
		if err == nil {
			re := regexp.MustCompile(`/gee/([a-zA-Z0-9_-]+)`)
			mo := re.FindSubmatch(path)
			if mo != nil {
				selectRepo = string(mo[1])
			}
		}
	}
	// Failing that, let's pick what the default_repo setting says.
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
	// Even after all of that, the user can still specify --upstream and --repository
	// on the command line.
	if upstream, err := flags.GetString("upstream"); (err != nil) && (upstream != "") {
		viper.Set("upstream", upstream)
	}
	if repository, err := flags.GetString("repository"); (err != nil) && (repository != "") {
		viper.Set("repository", repository)
	}
	// If, after all that, we still don't ahve the upstream/repository information, give up.
	if viper.GetString("upstream") == "" {
		Logger().Fatal(`Error: "upstream" is not configured.`)
	}
	if viper.GetString("repository") == "" {
		Logger().Fatal(`Error: "repository" is not configured.`)
	}
  config := RepoConfig{
    Select: selectRepo,
    Upstream: upstream,
    Repository: repository
  }
	Logger().Debugf("Selected: %q (%q/%q)", selectRepo, viper.GetString("upstream"), viper.GetString("repository"))
  return config
}
