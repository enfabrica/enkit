package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/machinist"
	"path/filepath"
)

type Config struct {
	Name     string
	Tags     []string
	DnsNames []string

	HostKeyLocation           string
	CaPublicKeyLocation       string
	SSHDConfigurationLocation string

	AutoRestartSSHD bool
	ReWriteConfigs  bool
	ms              *machinist.SharedFlags
	af              *client.AuthFlags

	// Pam Location configs
	// "/etc/security/pam_script_acct"
	PamSecurityLocation string
}

func (nf *Config) MachinistFlags() *machinist.SharedFlags {
	return nf.ms
}

func (nf *Config) ToModifiers() []NodeModifier {
	var toReturn []NodeModifier
	toReturn = append(toReturn,
		WithName(nf.Name),
		WithTags(nf.Tags),
		WithAuthFlags(nf.af),
	)
	return toReturn
}

func (nf *Config) NssConfig() *NssConf  {
	return &NssConf{
		DefaultShell: "/bin/bash",
	}
}

// HostCertificate will return the path of the HostCertificate based on the path set by HostKeyLocation
// for example /foo/bar.pem will output /foo/bar-cert.pub
func (nf *Config) HostCertificate() string {
	b := filepath.Base(nf.HostKeyLocation)
	bExt := filepath.Ext(b)
	rawName := b[0 : len(b)-len(bExt)]
	d := filepath.Dir(nf.HostKeyLocation)
	return filepath.Join(d, rawName+"-cert.pub")
}
