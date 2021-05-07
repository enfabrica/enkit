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

func NewAgentCommand(bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "agent [SubCommands] -- [Command]",
		Short: "commands for the enkit specific ssh-agent, anything passed in will execute with SSH_AUTH_SOCK and SSH_AGENT_PID set for the enkti agent.",
	}
	// Note the following is intended to be user friendly, identities here are cert principals
	c.AddCommand(NewRunAgentCommand(c, bf))
	c.AddCommand(NewPrintCommand(c, bf))
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

func NewRunAgentCommand(parent *cobra.Command, bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "run -- [COMMAND]",
		Short: "Runs the following command using the enkit ssh-agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAgentCommand(parent, bf, args)
		},
	}
	return c
}

func RunAgentCommand(command *cobra.Command, bf *client.BaseFlags, args []string) error {
	agent, err := kcerts.FindSSHAgent(bf.Local, bf.Log)
	if err != nil {
		return err
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	var kwargs []string
	if len(args) > 0 {
		kwargs = append([]string{"-c"}, strings.Join(args[:], " "))
	}
	cmd := exec.Command(shell, kwargs...)
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

func NewPrintCommand(parent *cobra.Command, bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "print",
		Short: "alias for agent run -- ssh-agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAgentCommand(parent, bf, []string{"ssh-agent"})
		},
	}
	return c
}
