package version

import (
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
	return rc
}

func (r *Root) Run(cmd *cobra.Command, args []string) error {
	fmt.Printf("Built from commit: %s\n", stamp.GitSha)
	if stamp.GitBranch != "master" {
		fmt.Printf("Branch is based on master commit %s\n", stamp.GitMasterSha)
	}
	fmt.Printf("Built from branch: %s\n", stamp.GitBranch)
	fmt.Printf("Builder: %s\n", stamp.BuildUser)
	fmt.Printf("Built at: %s\n", stamp.BuildTimestamp())
	fmt.Printf("Clean build: %v\n", stamp.IsClean())
	fmt.Printf("Official build: %v\n", stamp.IsOfficial())
	return nil
}
