package kauth

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"golang.org/x/crypto/ssh"
)

// SaveCredentials saves the passed in credentials to the current ssh-agent. If the credentials are empty, i.e.
// the EnkitCredentials only contain EnkitCredentials.Token, it will return nil as a NoOp.
func SaveCredentials(credentials *EnkitCredentials, store cache.Store, l logger.Logger, sshopts ...kcerts.SSHAgentModifier) error {
	if len(credentials.CaHosts) == 0 || credentials.SSHCertificate == nil || credentials.PrivateKey == nil {
		return nil
	}
	l.Infof("Saving Credentials")
	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(credentials.CAPublicKey))
	if err != nil {
		return fmt.Errorf("could not parse CA public key: %w", err)
	}
	sshDir, err := kcerts.FindSSHDir()
	if err != nil {
		return err
	}
	err = kcerts.AddSSHCAToClient(caPublicKey, credentials.CaHosts, sshDir)
	if err != nil {
		return err
	}
	agent, err := kcerts.PrepareSSHAgent(store, l, sshopts...)
	if err != nil {
		return err
	}
        // If the agent was loaded from the environment, Close() is a noop.
        // If the code just started an agent, we DO NOT WANT TO KILL IT!
        // Killing it will destroy the key we just fetched.
        //
        // With this defer in place, login only works when `enkit agent run` was
        // first started in the current bash session!
	//   defer agent.Close() // WRONG!!

	if err := agent.AddCertificates(credentials.PrivateKey, credentials.SSHCertificate); err != nil {
		return err
	}
	l.Infof("Successfully saved certificates to your local ssh agent")
	return nil
}
