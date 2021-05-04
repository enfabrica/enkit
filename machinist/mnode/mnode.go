package mnode

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/auth"
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
		fmt.Println("setting controller client")
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
func (n *Node) Enroll() error {
	if os.Geteuid() != 0 && n.config.RequireRoot {
		return errors.New("this command must be run as root since it touches the /etc/ssh directory")
	}
	pubKey, privKey, err := kcerts.GenerateED25519()
	if err != nil {
		return err
	}
	hcr := &auth.HostCertificateRequest{
		Hostcert: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ssh.MarshalAuthorizedKey(pubKey)}),
		Hosts:    n.config.DnsNames,
	}
	resp, err := n.AuthClient.HostCertificate(context.Background(), hcr)
	if err != nil {
		fmt.Println("error here")
		return err
	}
	if fName, exists := anyFileExist(
		n.config.CaPublicKeyLocation,
		n.config.HostKeyLocation, n.config.HostCertificate()); exists && !n.config.ReWriteConfigs {
		return fmt.Errorf("cannot rewrite %s because it exists and rewriting is disabled", fName)
	}

	enrollConfig := n.config.enrollConfigs
	// Pam Installer Steps
	n.Log.Infof("Executing Pam installation steps")
	if err := InstallLibPam(n.Log); err != nil {
		return err
	}
	if err := InstallPamSSHDFile(enrollConfig.PamSSHDLocation, n.Log); err != nil {
		return err
	}
	if err := InstallPamScript(enrollConfig.PamSecurityLocation, n.Log); err != nil {
		return err
	}

	//// Nss AutoUser Setup
	if err := InstallNssAutoUserConf(n.config.LibNssConfLocation, n.config.NssConfig()); err != nil {
		return err
	}
	if err := InstallNssAutoUser(n.Log); err != nil {
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
	n.Log.Infof("Writing Host Key")
	pemBytes, err := privKey.SSHPemEncode()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(n.config.HostKeyLocation, pemBytes, 0644); err != nil {
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
