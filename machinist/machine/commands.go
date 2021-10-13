package machine

import (
	"fmt"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func NewNodeCommand(common *config.Common) *cobra.Command {
	conf := &config.Node{
		Common: common,
	}
	c := &cobra.Command{
		Use: "node [OPTIONS] [SUBCOMMANDS]",
	}
	// Global Relate Flags
	c.PersistentFlags().StringArrayVar(&conf.Tags, "tags", []string{}, "the list of tags you want this node to have. Setting this will unset the cache")
	h, err := os.Hostname()
	if err != nil {
		panic(err) // Hostnames are important for ssh configuration, this is a valid panic.
	}
	c.PersistentFlags().StringVar(&conf.Name, "name", h, "the name of this node. If a node already exists with this name, polling the machinist server will fail")
	c.PersistentFlags().StringArrayVar(&conf.SSHPrincipals, "ssh-principals", []string{"localhost"}, "the list of ssh names you want this node to have, typically these line up with the dns aliases of the machine")

	c.AddCommand(NewEnrollCommand(conf))
	c.AddCommand(NewPollCommand(conf))
	c.AddCommand(NewSystemdCommand())
	return c
}

func NewEnrollCommand(conf *config.Node) *cobra.Command {
	c := &cobra.Command{
		Use:  "enroll [Name] [OPTIONS]",
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := New(WithConfig(conf))
			if err != nil {
				return err
			}
			return n.Enroll()
		},
	}
	// General Flags.
	c.PersistentFlags().BoolVar(&conf.RequireRoot, "require-root", true, "should the enroll command require root for execution")

	// NSS AutoUser flags.
	c.PersistentFlags().StringVar(&conf.LibNssConfLocation, "nss-autouser-conf", "/etc/nss-autouser.conf", "the file location of libnss autouser configuration file")

	// Pam Flags.
	c.PersistentFlags().StringVar(&conf.PamSecurityLocation, "pam-account-script-file", "/etc/security/pam_script_acct", "the location where to save the machinist host key")
	c.PersistentFlags().StringVar(&conf.PamSSHDLocation, "pam-sshd-file", "/etc/pam.d/sshd", "the location where to save PAM sshd configuration")

	// SSHD Related Flags.
	c.PersistentFlags().BoolVar(&conf.AutoRestartSSHD, "auto-restart-ssh", true, "if enroll is successful, auto restart sshd by calling service sshd-restart")
	c.PersistentFlags().StringVar(&conf.SSHDConfigurationLocation, "sshd-configuration-file", "/etc/ssh/sshd_config.d/machinist.conf", "the location where to save the machinist host key")
	c.PersistentFlags().StringVar(&conf.HostKeyLocation, "host-key-file", "/etc/ssh/machinist_host_key", "the location where to save the machinist host key, the signed certificate will be written to the same path with -cert.pub appended")
	c.PersistentFlags().StringVar(&conf.CaPublicKeyLocation, "ca-key-file", "/etc/ssh/machinist_ca.pub", "the file location of the CA's public key from the auth server. If the file already exists, defers to the rewrite flag")
	c.PersistentFlags().BoolVar(&conf.ReWriteConfigs, "rewrite", true, "rewrite HostKey and HostCert and TrustedCAKey if it already exists on the system")

	return c
}

func NewPollCommand(conf *config.Node) *cobra.Command {
	c := &cobra.Command{
		Use: "poll [SUBCOMMANDS] [OPTIONS]",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := New(WithConfig(conf))
			if err != nil {
				return err
			}
			if err = m.Init(); err != nil {
				return err
			}
			return m.BeginPolling()
		},
	}
	c.PersistentFlags().StringArrayVar(&conf.IpAddresses, "ips", []string{}, "the list of ip addresses bound to this machine")
	return c
}



type SystemdDConfig struct {
	User        string
	InstallPath string
	Command     string
}

func NewSystemdCommand() *cobra.Command {
	config := &SystemdDConfig{}
	c := &cobra.Command{
		Use:   "systemd -- [COMMAND]",
		Short: "Outputs a machinist.service compatible with systemd, using the passed in command to machinist",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := ParseSystemdTemplate(config.User, config.InstallPath, strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Println(res)
			return nil
		},
	}
	c.Flags().StringVar(&config.User, "user", os.Getenv("USER"), "the user to install the systemd command as, default to the current user")
	c.Flags().StringVar(&config.InstallPath, "install-path", "/usr/local/bin/machinist", "the installed path of the machinist binary")
	return c
}
