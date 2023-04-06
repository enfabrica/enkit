// Package machinecert provides the machine-cert subcommands to enkit.
package machinecert

import (
	"fmt"

	"github.com/enfabrica/enkit/lib/client"

	"github.com/spf13/cobra"
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	PublicKeyPath string
	PrivateKeyPath string
	SignedCertPath string
	SshdConfigPath string
}

func New(base *client.BaseFlags) (*Root, error) {
	root, err := NewRoot(base)
	if err != nil {
		return nil, err
	}

	root.AddCommand(NewPrint(root).Command)
	root.AddCommand(NewInstall(root).Command)

	return root, nil
}

func NewRoot(base *client.BaseFlags) (*Root, error) {
	rc := &Root{
		Command: &cobra.Command{
			Use:   "machine-cert",
			Short: "Commands for managing a machine's SSH certificate",
			Long:  `machine-cert - commands for managing a machine's SSH certificate`,
		},
		BaseFlags: base,
	}

	rc.PersistentFlags().StringVar(
		&rc.PublicKeyPath,
		"public-key-path",
		"/etc/ssh/machinist_host_key.pub",
		"Path to location where unsigned public key should be read from or installed to",
	)
	rc.PersistentFlags().StringVar(
		&rc.PrivateKeyPath,
		"private-key-path",
		"/etc/ssh/machinist_host_key",
		"Path to location where private key should be read from or installed to",
	)
	rc.PersistentFlags().StringVar(
		&rc.SignedCertPath,
		"private-key-path",
		"/etc/ssh/machinist_host_key",
		"Path to location where signed cert should be read from or installed to",
	)
	rc.PersistentFlags().StringVar(
		&rc.SshdConfigPath,
		"sshd-config-path",
		"/etc/ssh/sshd_config",
		"Path to sshd configuration to read/modify",
	)

	return rc, nil
}

type Print struct {
	*cobra.Command
	root *Root
}

func NewPrint(root *Root) *Print {
	command := &Print{
		Command: &cobra.Command{
			Use:     "print",
			Short:   "Print the machine's SSH certificate",
			Example: `  $ enkit machine-cert print`,
		},
		root: root,
	}

	command.Command.RunE = command.Run
	return command
}

func (p *Print) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("enkit machine-cert print not yet implemented")
}

type Install struct {
	*cobra.Command
	root *Root

	ExistingPublicKeyPath string
	ExistingPrivateKeyPath string
	Overwrite bool
	ConfigureSshd bool
	RestartSshd bool
}

func NewInstall(root *Root) *Install {
	command := &Install{
		Command: &cobra.Command{
			Use:   "install",
			Short: "Create and write a new SSH cert to this machine",
			// TODO: This example is probably wrong after this command is implemented
			Example: `  $ enkit machine-cert install`,
		},
		root: root,
	}

	command.Command.RunE = command.Run
	command.Flags().StringVar(
		&command.ExistingPublicKeyPath,
		"existing-public-key",
		"",
		"If set, use this public key instead of generating a new one",
	)
	command.Flags().StringVar(
		&command.ExistingPrivateKeyPath,
		"existing-private-key",
		"",
		"If set, use this private key instead of generating a new one",
	)
	command.Flags().BoolVar(
		&command.Overwrite,
		"overwrite",
		false,
		"If set, replace existing public key/private key/cert if they exist",
	)
	command.Flags().BoolVar(
		&command.ConfigureSshd,
		"configure-sshd",
		true,
		"If set, possibly modify sshd configuration to enable cert authentication",
	)
	command.Flags().BoolVar(
		&command.ConfigureSshd,
		"restart-sshd",
		true,
		"If set, restart sshd when configuration is modified automatically",
	)
	return command
}

func (i *Install) Run(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("enkit machine-cert install not yet implemented")
}
