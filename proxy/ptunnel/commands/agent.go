package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"strings"
)

type agentConfig struct {
	Print         bool
	RemainingArgs []string
}

func NewAgentCommand(bf *client.BaseFlags) *cobra.Command {
	agentConfig := &agentConfig{}
	c := &cobra.Command{
		Use:   "agent [SubCommands] -- [Command]",
		Short: "commands for the enkit specific ssh-agent, anything passed in will execute with SSH_AUTH_SOCK and SSH_AGENT_PID set for the enkti agent.",
	}
	// Note the following is intended to be user friendly, identities here are cert principals
	c.Flags().BoolVarP(&agentConfig.Print, "print", "p", false, "print the socket and PID of the running agent")
	c.AddCommand(NewRunAgentCommand(c, bf, agentConfig))
	c.AddCommand(NewListAgentCommand(bf))
	return c
}
func NewListAgentCommand(bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := kcerts.FindSSHAgent(bf.Local, bf.Log)
			if err != nil {
				return err
			}
			principals, err := agent.Principals()
			if err != nil {
				return err
			}
			for _, p := range principals {
				fmt.Printf("PKS: %s Identities: %v ValidFor: %s \n", p.MD5, p.Principals, p.ValidFor.String())
			}
			return nil
		},
	}
	return c
}

func NewRunAgentCommand(parent *cobra.Command, bf *client.BaseFlags, config *agentConfig) *cobra.Command {
	c := &cobra.Command{
		Use:   "run -- [COMMAND]",
		Short: "Runs the following command using the enkit ssh-agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAgentCommand(parent, bf, config, args)
		},
	}
	return c
}

func RunAgentCommand(command *cobra.Command, bf *client.BaseFlags, config *agentConfig, args []string) error {
	agent, err := kcerts.FindSSHAgent(bf.Local, bf.Log)
	if err != nil {
		return err
	}
	if config.Print {
		fmt.Printf("The enkit agent is running at socket %s \n", agent.Socket)
		fmt.Printf("The enkit agent's pid is %d \n", agent.PID)
	}
	cmd := exec.Command("sh", append([]string{"-c"}, strings.Join(args[:], " "))...)
	cmd.Stdout = command.OutOrStdout()
	cmd.Stderr = command.ErrOrStderr()
	cmd.Stdin = command.InOrStdin()
	cmd.Env = append(os.Environ(), agent.GetEnv()...)
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return kflags.NewStatusError(exit.ExitCode(), err)
		}
		return err
	}
	return nil
}
