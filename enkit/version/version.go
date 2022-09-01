package version

import (
	"errors"
	"fmt"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/stamp"

	"github.com/spf13/cobra"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags
}

func New(base *client.BaseFlags) *Root {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "version",
			Short:         "Show version info",
			SilenceUsage:  true,
			SilenceErrors: true,
			Long:          "version - command for getting the version of this tool",
		},
		BaseFlags: base,
	}
	rc.Command.RunE = rc.Run

	rc.AddCommand(NewHasFeature().Command)

	return rc
}

func (r *Root) Run(cmd *cobra.Command, args []string) error {
	fmt.Printf("Built from commit: %s\n", stamp.GitSha)
	if stamp.GitBranch != "master" {
		fmt.Printf("Branch is based on master commit %s\n", stamp.GitMasterSha)
	}
	fmt.Printf("Built from branch: %s\n", stamp.GitBranch)
	fmt.Printf("Builder: %s\n", stamp.BuildUser)
	fmt.Printf("Clean build: %v\n", stamp.IsClean())
	fmt.Printf("Official build: %v\n", stamp.IsOfficial())
	return nil
}

type HasFeature struct {
	*cobra.Command
}

func NewHasFeature() *HasFeature {
	c := &HasFeature {
		Command: &cobra.Command{
			Use: "has-feature <feature> [<feature> ...]",
			Short: "Interrogate if this enkit binary has the named feature/fix. Sets exit code to 0 for yes, non-zero for no.",
			Example: `  $ enkit version has-feature INFRA-1234-fix
	Checks for a fix for INFRA-1234.`,
		},
	}
	c.Command.RunE = c.Run
	return c
}

func (hf *HasFeature) Run(cmd *cobra.Command, args []string) error {
	hasAll := true
	for _, feature := range args {
		if !featuresContains(feature) {
			hasAll = false
			fmt.Printf("Feature not found: %s\n", feature)
		}
	}
	if !hasAll {
		return errors.New("not all features present in this enkit version")
	}
	return nil
}