package machinecert

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	apb "github.com/enfabrica/enkit/auth/proto"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/ssh"
)

func (i *Install) Run(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get connection to auth server
	conn, err := i.root.Connect()
	if err != nil {
		return fmt.Errorf("can't connect to auth server: %w", err)
	}
	authClient := apb.NewAuthClient(conn)

	// Check file overwriting
	created, overwritten, err := i.getAffectedFiles()
	if err != nil {
		return fmt.Errorf("while determining changed files: %w", err)
	}
	if len(overwritten) > 0 && !i.Overwrite {
		return fmt.Errorf("--overwrite not specified, but command would change files: %v", overwritten)
	}

	if err := createDirs(created); err != nil {
		return fmt.Errorf("ensuring directories exist for new files: %w", err)
	}

	privateKey, err := readFileOrEmpty(i.ExistingPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("while loading private key: %w", err)
	}
	publicKey, err := readFileOrEmpty(i.ExistingPublicKeyPath)
	if err != nil {
		return fmt.Errorf("while loading public key: %w", err)
	}

	if (publicKey != nil && privateKey == nil) || (publicKey == nil && privateKey != nil) {
		return fmt.Errorf("If one of [--existing-private-key, --existing-public-key] is supplied, both must be supplied")
	}
	if privateKey == nil || publicKey == nil {
		// Create new keypair
		pubKey, privKey, err := kcerts.GenerateED25519()
		if err != nil {
			return fmt.Errorf("failed to generate new keypair: %w", err)
		}
		publicKey = pubKey.Marshal()
		publicKeyAuthLine := ssh.MarshalAuthorizedKey(pubKey)

		// Write private key to disk
		privateKey, err = privKey.SSHPemEncode()
		if err != nil {
			return fmt.Errorf("while encoding private key: %w", err)
		}
		if err := ioutil.WriteFile(i.root.PrivateKeyPath, privateKey, 0600); err != nil {
			return fmt.Errorf("while writing private key: %w", err)
		}
		// Write public key to disk
		if err := ioutil.WriteFile(i.root.PublicKeyPath, publicKeyAuthLine, 0640); err != nil {
			return fmt.Errorf("while writing public key: %w", err)
		}
	}

	// Sign host cert
	principals := i.SshPrincipals
	if principals == nil {
		principals = append(principals, "localhost")
		hostname, err := os.Hostname()
		if err == nil {
			principals = append(principals, hostname)
		}
	}

	pubKey, err := ssh.ParsePublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("can't parse public key content: %w", err)
	}

	req := &apb.HostCertificateRequest{
		Hostcert: pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: ssh.MarshalAuthorizedKey(pubKey),
		}),
		Hosts: principals,
	}
	res, err := authClient.HostCertificate(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get cert signed by auth server: %w", err)
	}

	// Write host cert
	if err := ioutil.WriteFile(i.root.SignedCertPath, res.Signedhostcert, 0640); err != nil {
		return fmt.Errorf("while writing host cert key: %w", err)
	}

	// Write CA public key
	if err := ioutil.WriteFile(i.root.PublicCaKeyPath, res.Capublickey, 0640); err != nil {
		return fmt.Errorf("while writing CA public key: %w", err)
	}

	if i.ConfigureSshd {
		contents := fmt.Sprintf(`
HostKey %s
TrustedUserCAKeys %s
HostCertificate %s
`, i.root.PrivateKeyPath, i.root.PublicCaKeyPath, i.root.SignedCertPath)
		if err := ioutil.WriteFile(i.root.SshdConfigPath, []byte(strings.TrimSpace(contents)), 0600); err != nil {
			return fmt.Errorf("while writing sshd config: %w", err)
		}
	}

	if i.RestartSshd {
		out, err := exec.Command("systemctl", "restart", "sshd").CombinedOutput()
		if err != nil {
			return fmt.Errorf("sshd restart failed: %v\nOutput:\n%s", err, string(out))
		}
	}

	return nil
}

func (i *Install) getAffectedFiles() ([]string, []string, error) {
	newFiles := []string{}
	changedFiles := []string{}

	for _, path := range []string{i.root.PrivateKeyPath, i.root.PublicCaKeyPath, i.root.PublicKeyPath} {
		if _, err := os.Stat(path); err == nil {
			changedFiles = append(changedFiles, path)
		} else if errors.Is(err, os.ErrNotExist) {
			newFiles = append(newFiles, path)
		} else {
			return nil, nil, err
		}
	}

	return newFiles, changedFiles, nil
}

func createDirs(paths []string) error {
	for _, path := range paths {
		parent := filepath.Dir(path)
		if _, err := os.Stat(parent); errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(parent, 0755); err != nil {
				return fmt.Errorf("making parent dir %q: %w", parent, err)
			}
		}
	}
	return nil
}

func readFileOrEmpty(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("loading existing files not implemented")
	} else {
		return nil, fmt.Errorf("error reading file %q: %w", path, err)
	}
}
