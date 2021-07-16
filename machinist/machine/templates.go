package machine

import (
	"bytes"
	"errors"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/machinist/machinist_assets"
	"io/ioutil"
	"os/exec"
	"strings"
	"text/template"
)

func InstallLibPam(l logger.Logger) error {
	cmd := exec.Command("/bin/bash", "-c", string(machinist_assets.InstallLibPamScript))
	o, err := cmd.Output()
	l.Infof("output of installer is %s", string(o))
	return err
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

// fetchLibNSSAutoUser will fetch the nss_autouser  lib that's embedded. The current build exports one .a and one .so.
func fetchLibNSSAutoUser() ([]byte, error) {
	for k, v := range machinist_assets.AutoUserBinaries {
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
// Nss Installer Functions


// InstallNssAutoUserConf will read from the nssautouser.conf.gotmpl file, and output in /etc/nss-autouser.conf
func InstallNssAutoUserConf(path string, conf *NssConf) error {
	fileContent, err := ReadNssConf(conf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, fileContent, 0600)
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


// Pam Installer Functions
func InstallPamSSHDFile(path string, l logger.Logger) error {
	l.Infof("installing pam login file")
	return ioutil.WriteFile(path, machinist_assets.PamSSHDConfig, 0700)
}

func InstallPamScript(path string, l logger.Logger) error {
	l.Infof("Installing Pam Account Script")
	return ioutil.WriteFile(path, machinist_assets.PamScript, 0755)
}

func ParseSystemdTemplate(user, installPath, command string) (string, error) {
	tpl, err := template.New("machinist_service").Parse(string(machinist_assets.SystemdTemplate))
	if err != nil {
		return "", err
	}
	type ll struct {
		InstallPath string
		Command     string
		User        string
	}
	l := ll{
		User: user, InstallPath: installPath, Command: command,
	}
	var r []byte
	reader := bytes.NewBuffer(r)
	err = tpl.Execute(reader, l)
	return string(reader.Bytes()), err
}
