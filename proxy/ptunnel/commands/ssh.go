package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

type SSH struct {
	*cobra.Command
	*client.BaseFlags

	Tunnel     string
	Extra      string
	Subcommand string

	Proxy            string
	BufferSize       int
	SSH              string
	UseInternalAgent bool
}

func (r *SSH) Username() string {
	user, err := user.Current()
	if err != nil {
		return "<unknown>"
	}
	return user.Username
}

func IsValid(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if (info.Mode() & (os.ModeDir | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular)) != 0 {
		return fmt.Errorf("%s is not a regular file", filename)
	}
	return nil
}

func (r *SSH) Run(cmd *cobra.Command, args []string) error {
	if err := IsValid(r.Tunnel); err != nil {
		return kflags.NewUsageErrorf("Tunnel binary specified with --tunnel cannot be run: %w", err)
	}

	ssh := r.SSH
	if ssh == "" {
		ssh = "ssh"
	}

	found, err := exec.LookPath(ssh)
	if err != nil {
		return kflags.NewUsageErrorf("could not find '%s' binary: %w", ssh, err)
	}
	if err := IsValid(found); err != nil {
		return kflags.NewUsageErrorf("SSH binary cannot be run (fix with --ssh, -e, or changing your PATH): %w", err)
	}

	params := ""
	if r.Subcommand != "" {
		params += " " + r.Subcommand
	}
	if r.Proxy != "" {
		params += " --proxy=" + r.Proxy
	}
	if r.BufferSize != 0 {
		params += fmt.Sprintf(" --buffer-size=%d", r.BufferSize)
	}
	if r.Extra != "" {
		params += " " + r.Extra
	}

	args = append([]string{
		fmt.Sprintf("-oProxyCommand=%s%s %%h %%p", r.Tunnel, params),
	}, args...)

	ecmd := exec.Command(found, args...)
	ecmd.Stdin = os.Stdin
	ecmd.Stdout = os.Stdout
	ecmd.Stderr = os.Stderr

	if r.UseInternalAgent {
		agent, err := kcerts.FindSSHAgent(r.BaseFlags.Local, r.Log)
		if err != nil {
			return err
		}
		ecmd.Env = append(os.Environ(), agent.GetEnv()...)
	}


	if err := ecmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %s: %w", ecmd, err)
	}

	r.Log.Infof("user %s pid %d started - %s", r.Username(), ecmd.Process.Pid, cmd)
	err = ecmd.Wait()

	if err != nil {
		r.Log.Infof("user %s pid %d completed - %v", r.Username(), ecmd.Process.Pid, err)
		status, ok := err.(*exec.ExitError)
		if !ok {
			return err
		}
		return kflags.NewStatusError(status.ExitCode(), err)
	}
	r.Log.Infof("user %s pid %d completed - successful completion", r.Username(), ecmd.Process.Pid)
	return nil
}

func NewSSH(base *client.BaseFlags) *SSH {
	root := &SSH{
		Command: &cobra.Command{
			Use:           "ssh",
			Short:         "Configures tunnels for you to ssh in your corp infrastructure",
			Long:          `ssh - connects to your corp infrastructure using tunnels`,
			SilenceUsage:  true,
			SilenceErrors: true,
			Example: `  $ ... ssh 10.10.0.12
	Will run ssh to your host 10.10.0.12 through your default proxy.

  $ ... ssh user@10.10.0.12
	Same as above. Use it just like you were using the normal ssh command.

  $ ... ssh -- -p2222 user@10.10.0.12
	Pass option -p2222 to ssh. Note the '--', it is mandatory! It informs enkit that
	the following options are for ssh, not for the tunneling command.

  $ ... ssh --proxy=https://gw.corp.enfabrica.net -- -p2222 user@10.10.0.12
	Use a non default proxy to connect to your corp host.

  $ ... ssh --tunnel-extra="--browser-write-timeout=10s" -e "/bin/openssh" -- -p2222 user@10.10.0.12
	Pass some extra flags to the tunnel command, use a different ssh than the one that can
	be found in your path.
`,
		},
		BaseFlags: base,
	}
	root.RunE = root.Run

	root.Command.Flags().IntVar(&root.BufferSize, "buffer-size", 0, "Default read and write buffer size for window management. If 0, leave the default used by the tunnel")
	root.Command.Flags().StringVarP(&root.Proxy, "proxy", "p", "", "Full url of the proxy to connect to, must be specified")

	root.Command.Flags().StringVarP(&root.SSH, "ssh", "e", "", "Path to the SSH binary to use. If empty, one will be found for you")

	exec, _ := os.Executable()
	tcommand := ""
	if strings.HasSuffix(exec, "enkit") {
		tcommand = "tunnel"
	}

	root.Command.Flags().StringVar(&root.Tunnel, "tunnel", exec, "Path to the tunnel binary to use. Note that the binary will be invoked differently depending on the name of the binary")
	root.Command.Flags().StringVar(&root.Subcommand, "tunnel-command", tcommand, "Subcommand to use with the tunnel command. Defaults to empty if the tunnel command does not end with enkit")
	root.Command.Flags().StringVar(&root.Extra, "tunnel-extra", "", "Extra arguments to pass to the tunnel command")
	root.Command.Flags().BoolVar(&root.UseInternalAgent, "use-internal-agent", true, "Use the builtin agent that enkit manages")
	root.Command.AddCommand(NewAgentCommand(root.BaseFlags))
	return root
}
