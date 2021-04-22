package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/spf13/cobra"
)

func ShowAgent(baseFlags *client.BaseFlags) error {
	agent, err := kcerts.FindSSHAgent(baseFlags, baseFlags.Log)
	if err != nil {
		return err
	}
	fmt.Println("enkit agent's socket is ", agent.Socket)
	fmt.Println("enkit agent's PID is ", agent.PID)
	return nil
}

func NewShowAgentCommand(base *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use:  "show-agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ShowAgent(base)
		},
	}
	return c
}
