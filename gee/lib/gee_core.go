package lib

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func GetCurrentBranch() string {
	result := Runner().RunGit(Cmd{Quiet: true}, "rev-parse", "--abbrev-ref", "HEAD")
	if err := result.CheckExitCode(); err != nil {
		Logger().Fatalf("Error: %q", err)
	}
	return strings.TrimSpace(result.stdout.String())
}

// CheckInGeeRepo verifies that we're in a gee-controlled directory,
// or exits.
func CheckInGeeRepo() {
	path, err := os.Getwd()
	if err != nil {
		Logger().Fatalf("os.Getwd() failed: %q", err)
	}
	re := regexp.MustCompile(`/gee/([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)/`)
	if !re.MatchString(path) {
		Logger().Fatalf("Not a gee directory: %q", path)
	}
}

// Most configuration information is in viper, but the selection of the
// specific repository we're working with is a bit special:
type RepoConfig struct {
	Select     string // which repo block from the configuration file
	Upstream   string // the selected repository owner
	Repository string // the selected repository
	MainBranch string // the name of the main (master?) branch
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

// Return the absolute path to the current repository directory (not branch).
func (repoConfig *RepoConfig) GetRepoDir() string {
	home, err := homedir.Dir()
	if err != nil {
		Logger().Fatalf("Could not determine home directory: %q", err)
	}
	return fmt.Sprintf("%s/%s/%s", home, repoConfig.Upstream, repoConfig.Repository)
}

// DirectoryExists returns true if a path exists and is a directory.
func DirectoryExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// GetMainBranch determines the name of the main branch for this repository
// (usually main or master).
func (repoConfig *RepoConfig) GetMainBranch() string {
	// Return cached value:
	if repoConfig.MainBranch != "" {
		return repoConfig.MainBranch
	}
	// Guess based on what directories exist:
	repoDir := repoConfig.GetRepoDir()
	if DirectoryExists(repoDir + "/master") {
		repoConfig.MainBranch = "master"
		return repoConfig.MainBranch
	}
	if DirectoryExists(repoDir + "/main") {
		repoConfig.MainBranch = "main"
		return repoConfig.MainBranch
	}
	// Ask github
	if br, err := repoConfig.GetMainBranchNameFromGitHub(); err != nil {
		// Give up and assume "main"
		Logger().Debugf("Could not determine main branch: %q", err)
		return "main"
	} else {
		repoConfig.MainBranch = br
	}
	return repoConfig.MainBranch
}

// GetMainBranchNameFromGitHub attempts to inspect the upstream
// repository on github to determine what name of main branch
// (usually main or master).
func (repoConfig *RepoConfig) GetMainBranchNameFromGitHub() (string, error) {
	url := fmt.Sprintf("%s:%s/%s.git",
		viper.GetString("git_ssh_username"),
		repoConfig.Upstream,
		repoConfig.Repository)
	result := Runner().RunGit(Cmd{CanFail: true}, "remote", "show", url)
	if err := result.CheckExitCode(); err != nil {
		return "", err
	}

	re_head_branch := regexp.MustCompile(`HEAD branch: (\S+)`)
	scanner := bufio.NewScanner(&result.stdout)
	for scanner.Scan() {
		mo := re_head_branch.FindSubmatch(scanner.Bytes())
		if mo != nil {
			return string(mo[1]), nil
		}
	}
	return "", fmt.Errorf("Unparseable output from %q: %q", result.command, result.stdout)
}
