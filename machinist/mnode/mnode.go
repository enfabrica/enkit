package mnode

import (
	"context"
	"crypto/x509"
	"encoding/pem"
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

func (n *Node) Enroll(username string) error {
	fmt.Printf("node in enroll is %v \n", n)
	_, err := enauth.PerformLogin(n.AuthClient, n.Log, n.Repeater, username)
	if err != nil {
		return err
	}
	privKey, pubKey, err := kcerts.MakeKeys()
	if err != nil {
		return err
	}
	hcr := &auth.HostCertificateRequest{
		Hostcert: pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PUBLIC KEY", Bytes: ssh.MarshalAuthorizedKey(pubKey)}),
		Hosts:    []string{"localhost"},
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
	n.Log.Infof("Writing Host Key")
	encodedPrivKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privKey),
		},
	)
	if err := ioutil.WriteFile(n.config.HostKeyLocation, encodedPrivKey, 0600); err != nil {
		return err
	}
	n.Log.Infof("Writing Host Cert")
	if err := ioutil.WriteFile(n.config.HostCertificate(), resp.Signedhostcert, 0644); err != nil {
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
