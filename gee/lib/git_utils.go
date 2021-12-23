package lib

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"regexp"
	"strings"
)

func GetCurrentBranch() string {
	result := Runner().RunGit("rev-parse", "--abbrev-ref", "HEAD")
	if err := result.CheckExitCode(); err != nil {
		Logger().Fatalf("Error: %q", err)
	}
	return strings.TrimSpace(result.stdout.String())
}

func GetMainBranchNameFromGitHub() (string, error) {
	url := fmt.Sprintf("%s:%s/%s.git",
		viper.GetString("git_ssh_username"),
		viper.GetString("upstream"),
		viper.GetString("repository"))
	result := Runner().RunGit("remote", "show", url)
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
