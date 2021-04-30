package mnode

import (
	"bytes"
	"errors"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/machinist_assets"
	"io/ioutil"
	"strings"
	"text/template"
)

func InstallLibPam(){

}

func ReadSSHDContent(cafile, hostKey, hostCertificateFile string) ([]byte, error) {
	tpl, err := template.New("ssh_server").Parse(machinist_assets.SSHDTemplate)
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

// fetchLibNSSAutoUser will fetch the nss_autouser  lib that's embedded.
func fetchLibNSSAutoUser() ([]byte, error) {
	for k, v := range machinist_assets.Data {
		if strings.Contains(k, ".so") {
			return v, nil
		}
	}
	return nil, errors.New("the nssautouser has not been embedded properly")
}

// InstallNssAutoUser will attempt to install nssAutoUser if it can. Requires root. If not run as root or if not on
// support operating system, it will error out.
func InstallNssAutoUser(l logger.Logger) error {
	nssBinary, err := fetchLibNSSAutoUser()
	if err != nil {
		return err
	}
	l.Infof("Successfully fetched the binary, installing now")
	// TODO(adam): configure arm support for filepath writing and nss building
	if err = ioutil.WriteFile("/lib/x86_64-linux-gnu/libnss_autouser.so.2", nssBinary, 0600); err != nil {
		return err
	}
	out, err := ioutil.ReadFile("/etc/nsswitch.conf")
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "passwd:         files autouser systemd") {
		l.Warnf("Your /etc/nssswitch.conf file needs to have the following line \n " +
			"passwd:         files autouser systemd\n For the sanity of your system we do not automate this")
	}
	return err
}

type NssConf struct {
	DefaultShell string
	Shells       []struct {
		Home  string
		Shell string
		Match string
	}
}

// InstallNssAutoUserConf will read from the nssautouser.conf.gotmpl file, and output in /etc/nss-autouser.conf
func InstallNssAutoUserConf(conf *NssConf) error {
	fileContent, err := ReadNssConf(conf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/etc/nss-autouser.conf", fileContent, 0600)
}

func ReadNssConf(conf *NssConf) ([]byte, error) {
	tpl, err := template.New("nss_config").Parse(machinist_assets.NssConfig)
	if err != nil {
		return nil, err
	}
	var r []byte
	reader := bytes.NewBuffer(r)
	err = tpl.Execute(reader, conf)
	return reader.Bytes(), err
}
func InstallPamSSHDFile(l logger.Logger) error {
	l.Infof("installing pam login file")
	return ioutil.WriteFile("/etc/pam.d/sshd", []byte("account required  pam_script.so dir=/etc/security"), 0700)
}
func InstallPamScript(l logger.Logger, path string) error {
	l.Infof("Installing Pam script")
	return ioutil.WriteFile(path, machinist_assets.PamScript, 0755)
}

func InstallCAForSSHD(l logger.Logger, path string, ca []byte) error {
	l.Infof("Installing CA Public Key to %s", path)
	return ioutil.WriteFile(path, ca, 0644)
}
