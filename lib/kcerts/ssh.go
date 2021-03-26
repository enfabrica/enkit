package kcerts

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const CAPrefix = "@cert-authority"
const SSHDir = ".ssh"
const KnownHosts = "known_hosts"

func AddSSHCAToHost(publicKey ssh.PublicKey, hosts []string) error {
	return nil
}

func AddSSHCAToClient(publicKey ssh.PublicKey, hosts []string) error {
	hDir, err := homedir.Dir()
	if err != nil {
		return err
	}
	hDir = hDir + "/"
	if _, err := os.Stat(hDir + SSHDir); os.IsNotExist(err) {
		return fmt.Errorf("ssh directory %s does not exist, please create it", hDir+SSHDir)
	}
	qualifiedKnownHosts := hDir + SSHDir + "/" + KnownHosts
	if _, err := os.Stat(qualifiedKnownHosts); os.IsNotExist(err) {
		return fmt.Errorf("ssh authorized hosts file %s does not exist, please create it", qualifiedKnownHosts)
	}
	caPublic := string(ssh.MarshalAuthorizedKey(publicKey))
	existingKnownHostsContent, err := ioutil.ReadFile(qualifiedKnownHosts)
	if err != nil {
		return err
	}
	knownHostsFile, err := os.OpenFile(qualifiedKnownHosts, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer knownHostsFile.Close()
	for _, dns := range hosts {
		publicFormat := fmt.Sprintf("%s %s %s", CAPrefix, dns, caPublic)
		if !strings.Contains(string(existingKnownHostsContent), publicFormat) {
			_, err = knownHostsFile.WriteString(publicFormat)
			if err != nil {
				fmt.Println("error is not nil", err.Error())
				return err
			}
		}
	}
	return nil
}

//StartSSHAgent Will start the ssh agent in the interactive terminal if it isn't present already
func StartSSHAgent() error {
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		return nil
	}
	cmd := exec.Command("ssh-agent", "-s")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	reader := bufio.NewScanner(bytes.NewReader(out))
	for reader.Scan() {
		if strings.Contains(reader.Text(), "SSH_AUTH_SOCK") {
			afterSockString := strings.SplitN(reader.Text(), "SSH_AUTH_SOCK=",  2)
			socketPath := strings.Split(afterSockString[1], ";")
			os.Setenv("SSH_AUTH_SOCK", strings.TrimSpace(socketPath[0]))
			fmt.Println("set SSH_AUTH_SOCK to", os.Getenv("SSH_AUTH_SOCK"))
		}
	}
	return err
}
