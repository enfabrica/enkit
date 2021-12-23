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
	config := new(RepoConfig)
	// First, check for a --select flag on the command line:
	config.Select, _ = flags.GetString("select")
	// Failing that, let's see if we're already in a gee directory:
	if config.Select == "" {
		path, err := os.Getwd()
		if err == nil {
			re := regexp.MustCompile(`^.*?/gee/([a-zA-Z0-9_-]+)`)
			mo := re.FindStringSubmatch(path)
			if mo != nil {
				config.Select = string(mo[1])
			}
		}
	}
	// Failing that, let's pick what the default_repo setting says.
	if config.Select == "" {
		config.Select = viper.GetString("default_repo")
	}
	if config.Select != "" {
		if !viper.IsSet("repos." + config.Select) {
			fmt.Println("Error: repos." + config.Select + " is not in your config file.")
			os.Exit(1)
		}
		if upstream := viper.GetString("repos." + config.Select + ".upstream"); upstream != "" {
			config.Upstream = upstream
		}
		if repository := viper.GetString("repos." + config.Select + ".repository"); repository != "" {
			config.Repository = repository
		}
	} // config.Select
	// Even after all of that, the user can still specify --upstream and --repository
	// on the command line.
	if upstream, err := flags.GetString("upstream"); (err != nil) && (upstream != "") {
		config.Upstream = upstream
	}
	if repository, err := flags.GetString("repository"); (err != nil) && (repository != "") {
		config.Repository = repository
	}
	// If, after all that, we still don't ahve the upstream/repository information, give up.
	if config.Upstream == "" {
		Logger().Fatal(`Error: "upstream" is not configured.`)
	}
	if config.Repository == "" {
		Logger().Fatal(`Error: "repository" is not configured.`)
	}
	Logger().Debugf("Selected: %q (%q/%q)", config.Select, config.Upstream, config.Repository)
	return config
}
