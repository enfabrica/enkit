package mnode

import (
	"bytes"
	"text/template"
)

func ReadSSHDContent(cafile, hostKey, hostCertificateFile string) ([]byte, error) {
	tpl, err := template.New("ssh_server").Parse(SSHDTemplate)
	if err != nil {
		return nil, err
	}
	type localConfig struct {
		HostKeyFile         string
		TrustedCAFile       string
		HostCertificateFile string
	}
	l := localConfig{
		TrustedCAFile:       cafile,
		HostKeyFile:         hostKey,
		HostCertificateFile: hostCertificateFile,
	}
	var r []byte
	reader := bytes.NewBuffer(r)
	err = tpl.Execute(reader, l)
	return reader.Bytes(), err
}
