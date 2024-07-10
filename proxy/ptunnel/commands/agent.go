package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"strings"
)

type AgentCommandFlags struct {
	Base  *client.BaseFlags
	Agent *kcerts.SSHAgentFlags
}

func NewAgentCommand(bf *client.BaseFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "agent [SubCommands] -- [Command]",
		Short: "commands for the enkit specific ssh-agent, anything passed in will execute with SSH_AUTH_SOCK and SSH_AGENT_PID set for the enkti agent.",
	}
	flags := &AgentCommandFlags{
		Base:  bf,
		Agent: kcerts.SSHAgentDefaultFlags(),
	}
	flags.Agent.Register(&kcobra.FlagSet{c.PersistentFlags()}, "")

	// Note the following is intended to be user friendly, identities here are cert principals
	c.AddCommand(NewRunAgentCommand(c, flags))
	c.AddCommand(NewPrintCommand(c, flags))
	c.AddCommand(NewCshPrintCommand(c, flags))
	c.AddCommand(NewListAgentCommand(flags))
	return c
}
func NewListAgentCommand(flags *AgentCommandFlags) *cobra.Command {
	includeExt := false
	c := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := kcerts.PrepareSSHAgent(flags.Base.Local, kcerts.WithLogging(flags.Base.Log), kcerts.WithFlags(flags.Agent))
			if err != nil {
				return err
			}
			principals, err := agent.Principals()
			if err != nil {
				return err
			}
			for _, p := range principals {
				fmt.Printf("PKS: %s Identities: %v ValidFor: %s \n", p.MD5, p.Principals, p.ValidFor.String())
				if includeExt {
					for k, v := range p.Ext {
						fmt.Printf("\t Extentsion: %s: %s \n", k, v)
					}
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&includeExt, "ext", false, "include certificate extensions when printing out ssh certificates")
	return c
}

func NewRunAgentCommand(parent *cobra.Command, flags *AgentCommandFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "run -- [COMMAND]",
		Short: "Runs the following command using the enkit ssh-agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunAgentCommand(parent, flags, args)
		},
	}
	return c
}

func RunAgentCommand(command *cobra.Command, flags *AgentCommandFlags, args []string) error {
	agent, err := kcerts.PrepareSSHAgent(flags.Base.Local, kcerts.WithLogging(flags.Base.Log), kcerts.WithFlags(flags.Agent))
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

func NewPrintCommand(parent *cobra.Command, flags *AgentCommandFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "print",
		Short: "Prints out the enkit agent as if you ran ssh-agent -s, compatible with bourne shells",
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, err := kcerts.PrepareSSHAgent(flags.Base.Local, kcerts.WithLogging(flags.Base.Log), kcerts.WithFlags(flags.Agent))
			if err != nil {
				return err
			}
			defer agent.Close()
			fmt.Printf("SSH_AUTH_SOCK=%s; export SSH_AUTH_SOCK;\n", agent.State.Socket)
			if agent.State.PID != 0 {
				fmt.Printf("SSH_AGENT_PID=%d; export SSH_AGENT_PID;\necho Agent pid %d;\n", agent.State.PID, agent.State.PID)
			}
			return nil
		},
	}
	return c
}

func NewCshPrintCommand(parent *cobra.Command, flags *AgentCommandFlags) *cobra.Command {
        c := &cobra.Command{
                Use: "csh",
                Short: "Prints out the enkit agent as if you ran ssh-agent -c, compatible with c-shells",
                RunE: func(cmd *cobra.Command, args []string) error {
                        agent, err := kcerts.PrepareSSHAgent(flags.Base.Local, kcerts.WithLogging(flags.Base.Log), kcerts.WithFlags(flags.Agent))
                        if err != nil {
                            return err
                        }
                        defer agent.Close()
                        fmt.Printf("setenv SSH_AUTH_SOCK %s;\n", agent.State.Socket)
                        if agent.State.PID != 0 {
                            fmt.Printf("setenv SSH_AGENT_PID %d;\necho Agent pid %d;\n", agent.State.PID, agent.State.PID)
                        }
                        return nil
                },
        }
        return c
}
