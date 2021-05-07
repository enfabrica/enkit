package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/machinist"
	"github.com/spf13/cobra"
)

func NewRootCommand(bf *client.BaseFlags) *cobra.Command {

	config := &Config{
		bf:            bf,
		ms:            &machinist.SharedFlags{},
		enrollConfigs: &enrollConfigs{},
	}

	c := &cobra.Command{
		Use: "node [OPTIONS] [SUBCOMMANDS]",
	}
	factory := func() (*Node, error) {
		newN, err := New(config)
		if err != nil {
			return nil, err
		}
		return newN, err
	}

	// Global Relate Flags
	c.PersistentFlags().StringArrayVar(&config.Tags, "tags", []string{}, "the list of tags you want this node to have. Setting this will unset the cache")
	c.PersistentFlags().StringVar(&config.Name, "name", "no-name", "the name of this node. If a node already exists with this name, polling the machinist server will fail")
	c.PersistentFlags().StringArrayVar(&config.DnsNames, "dns-names", []string{"localhost"}, "the list of dns names you want this node to have")

	c.AddCommand(NewEnrollCommand(config.enrollConfigs, factory))

	return c
}

type enrollConfigs struct {
	RequireRoot bool

	LibNssConfLocation string

	// Pam Location configs
	// "/etc/security/pam_script_acct"
	PamSecurityLocation string
	PamSSHDLocation     string
	// SSHD Configs
	AutoRestartSSHD           bool
	CaPublicKeyLocation       string
	HostKeyLocation           string
	SSHDConfigurationLocation string
	ReWriteConfigs            bool
}

func NewEnrollCommand(config *enrollConfigs, factoryFunc FactoryFunc) *cobra.Command {
	c := &cobra.Command{
		Use:  "enroll [Name] [OPTIONS]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := factoryFunc()
			if err != nil {
				return err
			}
			return n.Enroll()
		},
	}
	// General Flags
	c.PersistentFlags().BoolVar(&config.RequireRoot, "require-root", true, "should the enroll command require root for execution")

	// NSS Autouser flags
	c.PersistentFlags().StringVar(&config.LibNssConfLocation, "nss-autouser-conf", "/etc/nss-autouser.conf", "the file location of libnss autouser configuration file")

	// Pam Flags
	c.PersistentFlags().StringVar(&config.PamSecurityLocation, "pam-account-script-file", "/etc/security/pam_script_acct", "the location where to save the machinist host key")
	c.PersistentFlags().StringVar(&config.PamSSHDLocation, "pam-sshd-file", "/etc/pam.d/sshd", "the location where to save PAM sshd configuration")

	// SSHD Related Flags
	c.PersistentFlags().BoolVar(&config.AutoRestartSSHD, "auto-restart-ssh", true, "if enroll is is successful, auto restart sshd by calling service sshd-restart")
	c.PersistentFlags().StringVar(&config.SSHDConfigurationLocation, "sshd-configuration-file", "/etc/ssh/sshd_config.d/machinist.conf", "the location where to save the machinist host key")
	c.PersistentFlags().StringVar(&config.HostKeyLocation, "host-key-file", "/etc/ssh/machinist_host_key", "the location where to save the machinist host key, the signed certificate will be written to the same path with -cert.pub appended")
	c.PersistentFlags().StringVar(&config.CaPublicKeyLocation, "ca-key-file", "/etc/ssh/machinist_ca.pub", "the file location of the CA's public key from the auth server. If the file already exists, defers to the rewrite flag")
	c.PersistentFlags().BoolVar(&config.ReWriteConfigs, "rewrite", true, "rewrite HostKey and HostCert and TrustedCAKey if it already exists on the system")

	return c
}
