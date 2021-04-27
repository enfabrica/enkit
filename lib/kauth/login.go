package kauth

import (
	"context"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/pkg/browser"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/ssh"
	"math/rand"
)

type EnkitCredentials struct {
	Token string
	// The below fields can be possibly empty if the auth server does not support CA certificates.
	CaHosts        []string
	CAPublicKey    string
	PrivateKey     ed25519.PrivateKey
	SSHCertificate *ssh.Certificate
}

// PerformLogin will login with the provider auth client, retry and logger. It does not care about the cache.
// If you wish to save the result, please call SaveCredentials
func PerformLogin(authClient auth.AuthClient, l logger.Logger, repeater *retry.Options, username, domain string) (*EnkitCredentials, error) {
	rng := rand.New(srand.Source)
	pubBox, privBox, err := box.GenerateKey(rng)
	if err != nil {
		return nil, err
	}
	sshPub, sshPriv, err := kcerts.MakeKeys(kcerts.GenerateED25519)
	if err != nil {
		return nil, err
	}
	areq := &auth.AuthenticateRequest{
		Key:    (*pubBox)[:],
		User:   username,
		Domain: domain,
	}
	l.Infof("Retrieving authentication url.")
	ares, err := authClient.Authenticate(context.TODO(), areq)
	if err != nil {
		return nil, fmt.Errorf("Could not contact the authentication server. Is your connectivity working? Is the server up?\nFor debugging: %w", err)
	}
	l.Infof("Authentication url is %s, attempting to open with your Os's default browser", ares.Url)
	// browser.OpenURL blocks depending on permissions level and OS. By running it in a goroutine, we ensure that
	// the login process does not get stuck waiting for the browser window to be closed.
	go func() {
		if err := browser.OpenURL(ares.Url); err != nil {
			l.Warnf("Could not open auth url in default browser, you might have to navigate there yourself")
		}
	}()
	servPub, err := common.KeyFromSlice(ares.Key)
	if err != nil {
		return nil, fmt.Errorf("server provided invalid key - please retry - %s", err)
	}
	treq := &auth.TokenRequest{
		Url:       ares.Url,
		Publickey: pem.EncodeToMemory(&pem.Block{Type: "EC PUBLIC KEY", Bytes: sshPub}),
	}
	var tres *auth.TokenResponse
	if err := repeater.Run(func() error {
		l.Infof("Polling to retrieve token.")
		t, err := authClient.Token(context.TODO(), treq)
		if err != nil {
			l.Infof("Polling failed - %v - retrying in %s", err, repeater.Wait)
			return err
		}
		l.Infof("Polling succeeded - decrypting token")
		tres = t
		return nil
	}); err != nil {
		return nil, err
	}
	nonce, err := common.NonceFromSlice(tres.Nonce)
	if err != nil {
		return nil, fmt.Errorf("server returned invalid nonce, please try again - %s", err)
	}
	decrypted, ok := box.Open(nil, tres.Token, nonce.ToByte(), servPub.ToByte(), privBox)
	if !ok {
		return nil, fmt.Errorf("server returned invalid nonce, please try again - %s", err)
	}
	p, _, _, _, err := ssh.ParseAuthorizedKey(tres.Cert)
	if err != nil {
		return nil, err
	}
	cert, ok := p.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("public key sent back is not a valid ssh certificate, but was present")
	}
	castedPrivKey, ok := sshPriv.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key was not ed25519, we should never reach here")
	}
	return &EnkitCredentials{
		Token:          string(decrypted),
		CAPublicKey:    string(tres.Capublickey),
		CaHosts:        tres.Cahosts,
		PrivateKey:     castedPrivKey,
		SSHCertificate: cert,
	}, nil
}
