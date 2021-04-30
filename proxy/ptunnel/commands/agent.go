package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/spf13/cobra"
)

type agentConfig struct {
	Print          bool
	ListIdentities bool
}

func NewAgentCommand(bf *client.BaseFlags) *cobra.Command {
	agentConfig := &agentConfig{}
	c := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAgentCommand(bf, agentConfig)
		},
		Use:   "agent",
		Short: "commands for the enkit specific ssh-agent",
	}
	// Note the following is intended to be user friendly, identities here are cert principals
	c.Flags().BoolVarP(&agentConfig.ListIdentities, "list-identities", "l", false, "list the identities loaded current in the agent")
	c.Flags().BoolVarP(&agentConfig.Print, "print", "p", false, "print the socket and PID of the running agent")
	return c
}

func RunAgentCommand(bf *client.BaseFlags, config *agentConfig) error {
	agent, err := kcerts.FindSSHAgent(bf.Local, bf.Log)
	if err != nil {
		return err
	}
	if config.Print {
		fmt.Printf("The enkit agent is running at socket %s \n", agent.Socket)
		fmt.Printf("The enkit agent's pid is %d \n", agent.PID)

	}
	if config.ListIdentities {
		principals, err := agent.Principals()
		if err != nil {
			return err
		}
		for _, p := range principals {
			fmt.Printf("PKS: %s Identities: %v ValidFor: %s \n", p.MD5, p.Principals, p.ValidFor.String())
		}
	}
	return nil
}
