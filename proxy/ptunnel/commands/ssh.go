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
	"os/user"
	"strings"
)

type proxyMapping struct {
	substring string
	proxy     string
}

type SSH struct {
	*cobra.Command
	*client.BaseFlags
	AgentFlags *kcerts.SSHAgentFlags

	Tunnel     string
	Extra      string
	Subcommand string

	Proxy            string
	proxyList        []string
	ProxyMap         []proxyMapping
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

// parseFlags performs extra processing/validation on command-line flags as a
// pre-run step.
func (r *SSH) parseFlags(cmd *cobra.Command, args []string) error {
	for _, s := range r.proxyList {
		pair := strings.SplitN(s, "=", 2)
		if len(pair) != 2 {
			return fmt.Errorf("%q is not a valid proxy mapping; expected key=value", s)
		}
		r.ProxyMap = append(r.ProxyMap, proxyMapping{substring: pair[0], proxy: pair[1]})
	}
	return nil
}

// proxyFlag returns a formatted tunnel proxy flag that can be appended to an
// existing commandline.
func proxyFlag(proxy string) string {
	return " --proxy=" + proxy
}

// chooseProxy returns an appropriate proxy flag for a target passed somewhere
// in args, or the empty string if no proxy is to be explicitly set.
func (r *SSH) chooseProxy(args []string) string {
	for _, arg := range args {
		for _, entry := range r.ProxyMap {
			if strings.Contains(arg, entry.substring) {
				r.Log.Infof("Selected proxy %q for host %q", entry.proxy, arg)
				return proxyFlag(entry.proxy)
			}
		}
	}
	if r.Proxy != "" {
		r.Log.Infof("Selected default proxy %q for host in %v", r.Proxy, args)
		return proxyFlag(r.Proxy)
	}
	r.Log.Warnf("No proxy found for host in %v and no default proxy set; omitting --proxy flag", args)
	return ""
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
	params += r.chooseProxy(args)
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
		agent, err := kcerts.PrepareSSHAgent(r.BaseFlags.Local, kcerts.WithLogging(r.BaseFlags.Log), kcerts.WithFlags(r.AgentFlags))
		if err != nil {
			return err
		}
		ecmd.Env = append(os.Environ(), agent.GetEnv()...)
	}

	if err := ecmd.Start(); err != nil {
		return fmt.Errorf("failed to start command %s: %w", ecmd, err)
	}

	r.Log.Infof("user %s pid %d started - %s", r.Username(), ecmd.Process.Pid, ecmd)
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

	$ ... ssh --proxy-map=.mtv=https://gw.corp.enfabrica.net,.foo=https://example.com -- p2222 user@machine-name.foo
	Select from a few proxies based on string matching in the target address.
	In this case https://example.com will be selected as the proxy, as the
	target contains the string '.foo'.

  $ ... ssh --tunnel-extra="--browser-write-timeout=10s" -e "/bin/openssh" -- -p2222 user@10.10.0.12
	Pass some extra flags to the tunnel command, use a different ssh than the one that can
	be found in your path.
`,
		},
		BaseFlags:  base,
		AgentFlags: kcerts.SSHAgentDefaultFlags(),
	}
	root.PreRunE = root.parseFlags
	root.RunE = root.Run

	root.Command.Flags().IntVar(&root.BufferSize, "buffer-size", 0, "Default read and write buffer size for window management. If 0, leave the default used by the tunnel")
	root.Command.Flags().StringVarP(&root.Proxy, "proxy", "p", "", "Full url of the proxy to connect to, must be specified")
	root.Command.Flags().StringSliceVar(
		&root.proxyList,
		"proxy-map",
		nil,
		"Map of suffix=gateway pairs to use for choosing proxy. Overrides --proxy when an entry matches any of the targets. If set, --proxy is used as the default fallback when no entries match.",
	)
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
	root.AgentFlags.Register(&kcobra.FlagSet{root.Command.Flags()}, "")

	return root
}
