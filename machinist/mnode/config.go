package mnode

import (
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/machinist"
	"path/filepath"
)

type Config struct {
	Name          string
	Tags          []string
	SSHPrincipals []string

	ms              *machinist.SharedFlags
	bf              *client.BaseFlags
	*enrollConfigs
}

func (nf *Config) MachinistFlags() *machinist.SharedFlags {
	if nf.ms == nil {
		nf.ms = &machinist.SharedFlags{}
	}
	return nf.ms
}

func (nf *Config) ToModifiers() []NodeModifier {
	var toReturn []NodeModifier
	toReturn = append(toReturn,
		WithName(nf.Name),
		WithTags(nf.Tags),
	)
	return toReturn
}

func (nf *Config) NssConfig() *NssConf {
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
