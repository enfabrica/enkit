package machine

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/machinist/config"
	"github.com/enfabrica/enkit/machinist/polling"
	mpb "github.com/enfabrica/enkit/machinist/rpc"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
)

type Machine struct {
	MachinistClient mpb.ControllerClient
	AuthClient      auth.AuthClient
	Repeater        *retry.Options
	Log             logger.Logger

	// Dial func will override any existing options to connect
	DialFunc func() (*grpc.ClientConn, error)

	*config.Node
}

func (n *Machine) MachinistCommon() *config.Common {
	return n.Common
}

func (n *Machine) Init() error {
	if n.DialFunc != nil {
		conn, err := n.DialFunc()
		if err != nil {
			return err
		}
		n.MachinistClient = mpb.NewControllerClient(conn)
		return nil
	}
	h := n.ControlPlaneHost
	p := n.ControlPlanePort
	conn, err := grpc.Dial(net.JoinHostPort(h, strconv.Itoa(p)), grpc.WithInsecure())
	if err != nil {
		return err
	}
	n.MachinistClient = mpb.NewControllerClient(conn)
	return nil
}

func (n *Machine) BeginPolling() error {
	ctx := context.Background()
	return goroutine.WaitFirstError(
		func() error {
			return polling.SendRegisterRequests(ctx, n.MachinistClient, n.Node)
		},
		func() error {
			return polling.SendKeepAliveRequest(ctx, n.MachinistClient)
		},
		func() error {
			return polling.SendMetricsRequest(ctx, n.Node)
		},
	)
}

// TODO(adam): perform rollbacks if enroll fails
func (n *Machine) Enroll() error {
	if os.Geteuid() != 0 && n.RequireRoot {
		return errors.New("this command must be run as root since it touches the /etc/ssh directory")
	}
	pubKey, privKey, err := kcerts.GenerateED25519()
	if err != nil {
		return err
	}
	hcr := &auth.HostCertificateRequest{
		Hostcert: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ssh.MarshalAuthorizedKey(pubKey)}),
		Hosts:    n.SSHPrincipals,
	}
	resp, err := n.AuthClient.HostCertificate(context.Background(), hcr)
	if err != nil {
		return err
	}
	if !n.ReWriteConfigs {
		if err = anyFileExist(
			n.CaPublicKeyLocation,
			n.HostKeyLocation, n.HostCertificate()); err != nil {
			return fmt.Errorf("rewriting configs are disabled, failed with err(s): %v", err)
		}
	}

	// Pam Installer Steps
	n.Log.Infof("Executing Pam installation steps")
	if err := InstallLibPam(n.Log); err != nil {
		return err
	}
	if err := InstallPamSSHDFile(n.PamSSHDLocation, n.Log); err != nil {
		return err
	}
	if err := InstallPamScript(n.PamSecurityLocation, n.Log); err != nil {
		return err
	}

	//// Nss AutoUser Setup
	if err := InstallNssAutoUserConf(n.LibNssConfLocation, &NssConf{
		DefaultShell: "/bin/bash",
	}); err != nil {
		return err
	}
	if err := InstallNssAutoUser(n.Log); err != nil {
		return err
	}

	// SSHD installer steps
	if err := os.MkdirAll(filepath.Dir(n.SSHDConfigurationLocation), os.ModePerm); err != nil {
		return err
	}
	sshdConfigContent, err := ReadSSHDContent(n.CaPublicKeyLocation, n.HostKeyLocation, n.HostCertificate())
	if err != nil {
		return err
	}
	n.Log.Infof("Writing SSHD Configuration")
	if err := ioutil.WriteFile(n.SSHDConfigurationLocation, sshdConfigContent, 0644); err != nil {
		return err
	}
	n.Log.Infof("Writing CA Public Key Configuration")
	if err := ioutil.WriteFile(n.CaPublicKeyLocation, resp.Capublickey, 0644); err != nil {
		return err
	}
	n.Log.Infof("Writing Host Cert")
	if err := ioutil.WriteFile(n.HostCertificate(), resp.Signedhostcert, 0644); err != nil {
		return err
	}
	n.Log.Infof("Writing Host Key")
	pemBytes, err := privKey.SSHPemEncode()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(n.HostKeyLocation, pemBytes, 0644); err != nil {
		return err
	}
	return nil
}

func anyFileExist(names ...string) error {
	var errs []error
	for _, name := range names {
		if _, err := os.Stat(name); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return multierror.New(errs)
}
