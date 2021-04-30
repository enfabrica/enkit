package mnode

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/enauth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	machinist_rpc "github.com/enfabrica/enkit/machinist/rpc/machinist"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type Node struct {
	MachinistClient machinist_rpc.ControllerClient
	AuthClient      auth.AuthClient
	Repeater        *retry.Options
	Log             logger.Logger

	// Dial func will override any existing options to connect
	DialFunc func() (*grpc.ClientConn, error)

	config *Config
}

func (n *Node) Init() error {
	if n.DialFunc != nil {
		conn, err := n.DialFunc()
		if err != nil {
			return err
		}
		n.MachinistClient = machinist_rpc.NewControllerClient(conn)
		return nil
	}
	panic("not implemented yet")
}

func (n *Node) BeginPolling() error {
	ctx := context.Background()
	pollStream, err := n.MachinistClient.Poll(ctx)
	if err != nil {
		return err
	}
	initialRequest := &machinist_rpc.PollRequest{
		Req: &machinist_rpc.PollRequest_Register{
			Register: &machinist_rpc.ClientRegister{
				Name: n.config.Name,
				Tag:  n.config.Tags,
			},
		},
	}
	if err := pollStream.Send(initialRequest); err != nil {
		return fmt.Errorf("unable to send initial request: %w", err)
	}
	for {
		select {
		case <-time.After(1 * time.Second):
			pollReq := &machinist_rpc.PollRequest{
				Req: &machinist_rpc.PollRequest_Ping{
					Ping: &machinist_rpc.ClientPing{
						Payload: []byte(``),
					},
				},
			}
			if err := pollStream.Send(pollReq); err != nil {
				return fmt.Errorf("unable to send poll req: %w", err)
			}
		}
	}
}
// Todo(adam): perform rollbacks if enroll fails
func (n *Node) Enroll(username string) error {
	if os.Geteuid() != 0 {
		return errors.New("this command must be run as root since it touches the /etc/ssh directory")
	}
	pubKey, privKey, err := kcerts.GenerateED25519()
	if err != nil {
		return err
	}
	hcr := &auth.HostCertificateRequest{
		Hostcert: ssh.MarshalAuthorizedKey(pubKey),
		Hosts:    n.config.DnsNames,
	}
	resp, err := n.AuthClient.HostCertificate(context.Background(), hcr)
	if err != nil {
		return err
	}
	if fName, exists := anyFileExist(
		n.config.CaPublicKeyLocation,
		n.config.HostKeyLocation, n.config.HostCertificate()); exists && !n.config.ReWriteConfigs {
		return fmt.Errorf("cannot rewrite %s because it exists and rewriting is disabled", fName)
	}
	// Pam Installer Steps
	n.Log.Infof("Executing Pam installation steps")
	InstallLibPam()
	if err := InstallPamSSHDFile(n.Log); err != nil {
		return err
	}
	if err := InstallPamScript(n.Log); err != nil {
		return err
	}

	// SSHD installer steps

	if err := os.MkdirAll(filepath.Dir(n.config.SSHDConfigurationLocation), os.ModePerm); err != nil {
		return err
	}
	sshdConfigContent, err := ReadSSHDContent(n.config.CaPublicKeyLocation, n.config.HostKeyLocation, n.config.HostCertificate())
	if err != nil {
		return err
	}
	n.Log.Infof("Writing SSHD Configuration")
	if err := ioutil.WriteFile(n.config.SSHDConfigurationLocation, sshdConfigContent, 0644); err != nil {
		return err
	}
	n.Log.Infof("Writing CA Public Key Configuration")
	if err := ioutil.WriteFile(n.config.CaPublicKeyLocation, resp.Capublickey, 0644); err != nil {
		return err
	}

	n.Log.Infof("Writing Host Cert")
	if err := ioutil.WriteFile(n.config.HostCertificate(), resp.Signedhostcert, 0644); err != nil {
		return err
	}
	if err := InstallNssAutoUserConf(n.config.NssConfig()); err != nil {
		return err
	}
	if err := InstallNssAutoUser(n.Log); err != nil {
		return err
	}

	return nil
}

func anyFileExist(names ...string) (string, bool) {
	for _, name := range names {
		if _, err := os.Stat(name); err != nil {
			if os.IsNotExist(err) {
				continue
			}
		}
		return name, true
	}
	return "", false
}
