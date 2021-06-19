package config

import (
	"github.com/enfabrica/enkit/lib/client"
	"path/filepath"
)

type Node struct {
	Name          string
	Tags          []string
	SSHPrincipals []string
	IpAddresses   []string

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



	Root   *client.BaseFlags
	*Common
}


// HostCertificate will return the path of the HostCertificate based on the path set by HostKeyLocation
// for example /foo/bar.pem will output /foo/bar-cert.pub
func (nf *Node) HostCertificate() string {
	b := filepath.Base(nf.HostKeyLocation)
	bExt := filepath.Ext(b)
	rawName := b[0 : len(b)-len(bExt)]
	d := filepath.Dir(nf.HostKeyLocation)
	return filepath.Join(d, rawName+"-cert.pub")
}
