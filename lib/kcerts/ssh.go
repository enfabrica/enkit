package kcerts

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const CAPrefix = "@cert-authority"
const SSHDir = ".ssh"
const KnownHosts = "known_hosts"

const SSHCacheKey = "enkit_ssh_cache_key"

// FindSSHDir will find the users ssh directory based on $HOME. If $HOME/.ssh does not exist
// it will attempt to create it.
func FindSSHDir() (string, error) {
	hDir, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("could not find the home directory: %w", err)
	}
	sshDir := filepath.Join(hDir, SSHDir)
	if err := os.Mkdir(sshDir, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("could not create directory %s: %w", sshDir, err)
	}
	return sshDir, nil
}

// AddSSHCAToClient adds a public key to the $HOME/.ssh/known_hosts in the ssh-cert x509.1 format.
// For each entry, it adds an additional line and does not concatenate
func AddSSHCAToClient(publicKey ssh.PublicKey, hosts []string, sshDir string) error {
	caPublic := string(ssh.MarshalAuthorizedKey(publicKey))
	knownHosts := filepath.Join(sshDir, KnownHosts)
	knownHostsFile, err := os.OpenFile(knownHosts, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("could not create known_hosts file: %w", err)
		}
		return err
	}
	existingKnownHostsContent, err := ioutil.ReadAll(knownHostsFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", knownHosts, err)
	}
	defer knownHostsFile.Close()
	for _, dns := range hosts {
		// caPublic terminates with a '\n', added by ssh.MarshalAuthorizedKey
		publicFormat := fmt.Sprintf("%s %s %s", CAPrefix, dns, caPublic)
		if strings.Contains(string(existingKnownHostsContent), publicFormat) {
			continue
		}
		_, err = knownHostsFile.WriteString(publicFormat)
		if err != nil {
			return fmt.Errorf("could not add key %s to known_hosts file: %w", publicFormat, err)
		}
	}
	return nil
}

type sshCache struct {
	PID       int       `json:"pid"`
	Sock      string    `json:"sock"`
	TimeStamp time.Time `json:"time_stamp"`
}

// FindSSHAgent Will start the ssh agent in the interactive terminal if it isn't present already as an environment variable
// Currently only outputs the env and does not persist it across terminals
func FindSSHAgent(store cache.Store, ttl time.Duration) (string, int, error) {
	envSSHSock := os.Getenv("SSH_AUTH_SOCK")
	envSSHPID := os.Getenv("SSH_AGENT_PID")
	if envSSHSock != "" && envSSHPID != "" {
		fmt.Println("found in env")
		pid, err := strconv.Atoi(envSSHPID)
		if err != nil {
			return "", 0, fmt.Errorf("%s is not a valid pid: %w", envSSHPID, err)
		}
		return envSSHSock, pid, nil
	}
	// Currently the cache never errors out on Existing. therefore ignoring errors
	sshEnkitCache, isFresh, err := store.Get(SSHCacheKey)

	if err != nil {
		return "", 0, fmt.Errorf("error fetching cache: %w", err)
	}

	if isFresh {
		f, err := os.OpenFile(filepath.Join(sshEnkitCache, "ssh.json"), os.O_RDWR, 0750)
		if err != nil && !os.IsNotExist(err) {
			return "", 0, fmt.Errorf("error opening cache: %w", err)
		}
		if err == nil {
			cacheContent, err := ioutil.ReadAll(f)
			if err != nil {
				return "", 0, fmt.Errorf("error reading cache: %w", err)
			}
			var sshCache sshCache
			if err := json.Unmarshal(cacheContent, &sshCache); err != nil {
				return "", 0, fmt.Errorf("error deserializing cache: %w", err)
			}
			if time.Now().Before(sshCache.TimeStamp) {
				return sshCache.Sock, sshCache.PID, nil
			}
			err = store.Purge(sshEnkitCache)
			if err != nil {
				return "", 0, fmt.Errorf("error clearing the cache %w", err)
			}
			sshEnkitCache, _, err = store.Get(SSHCacheKey)
			if err != nil {
				return "", 0, fmt.Errorf("error refetching cache: %w", err)
			}
		}
	}

	cmd := exec.Command("ssh-agent", "-s")
	out, err := cmd.Output()
	if err != nil {
		return "", 0, err
	}

	sockR := regexp.MustCompile("(?m)SSH_AUTH_SOCK=([^;\\n]*)")
	pidR := regexp.MustCompile("(?m)SSH_AGENT_PID=([0-9]*)")
	resultSock := string(sockR.Find(out))
	resultPID := string(pidR.Find(out))

	rawSock := strings.Split(resultSock, "=")
	rawPId := strings.Split(resultPID, "=")

	if len(rawPId) != 2 || len(rawSock) != 2 {
		return "", 0, fmt.Errorf("not a valid pid or agent sock, %v %v", rawSock, rawPId)
	}
	// The second element after splitting is the raw value we want
	pid, err := strconv.Atoi(rawPId[1])
	if err != nil {
		return "", 0, fmt.Errorf("error processing ssh agent pid %s: %w", string(resultPID), err)
	}
	cacheToWrite := &sshCache{
		Sock:      rawSock[1],
		PID:       pid,
		TimeStamp: time.Now().Add(ttl),
	}
	b, err := json.Marshal(cacheToWrite)
	if err != nil {
		return "", 0, fmt.Errorf("error marshalling cache: %w", err)
	}
	err = ioutil.WriteFile(filepath.Join(sshEnkitCache, "ssh.json"), b, 0750)
	if err != nil {
		return "", 0, fmt.Errorf("error writing to file: %w", err)
	}
	_, err = store.Commit(sshEnkitCache)
	return rawSock[1], pid, err
}

// GenerateUserSSHCert will sign and return credentials based on the CA signer and given parameters
// to generate a user cert, certType must be 1, and host certs ust have certType 2
func GenerateUserSSHCert(ca ssh.Signer, certType uint32, principals []string, ttl time.Duration) (*rsa.PrivateKey, *ssh.Certificate, error) {
	priv, pub, err := makeKeys()
	if err != nil {
		return priv, nil, err
	}
	from := time.Now().UTC()
	to := time.Now().UTC().Add(ttl * time.Hour)
	cert := &ssh.Certificate{
		CertType:        certType,
		Key:             pub,
		ValidAfter:      uint64(from.Unix()),
		ValidBefore:     uint64(to.Unix()),
		ValidPrincipals: principals,
		Permissions:     ssh.Permissions{},
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, nil, err
	}
	return priv, cert, nil
}

func makeKeys() (*rsa.PrivateKey, ssh.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, err
}
