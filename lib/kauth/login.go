package kauth

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/auth/common"
	apb "github.com/enfabrica/enkit/auth/proto"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/pkg/browser"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/ssh"
	"math/rand"
)

func init() {
	browser.Stdout = nil
	browser.Stderr = nil
}

type EnkitCredentials struct {
	Token string
	// The below fields can be possibly empty if the auth server does not support CA certificates.
	CaHosts        []string
	CAPublicKey    string
	PrivateKey     kcerts.PrivateKey
	SSHCertificate *ssh.Certificate
}

// PerformLogin will login with the provider auth client, retry and logger. It does not care about the cache.
// If you wish to save the result, please call SaveCredentials
func PerformLogin(authClient apb.AuthClient, l logger.Logger, repeater *retry.Options, rng *rand.Rand, username, domain string) (*EnkitCredentials, error) {
	pubBox, privBox, err := box.GenerateKey(rng)
	if err != nil {
		return nil, err
	}
	areq := &apb.AuthenticateRequest{
		Key:    (*pubBox)[:],
		User:   username,
		Domain: domain,
	}
	l.Infof("Retrieving authentication url.")
	ares, err := authClient.Authenticate(context.TODO(), areq)
	if err != nil {
		return nil, fmt.Errorf("Could not contact the authentication server. Is your connectivity working? Is the server up?\nFor debugging: %w", err)
	}
	if username != "" {
		fmt.Printf("Dear %s, please visit:\n\n", username)
	} else {
		fmt.Printf("Kind human, please visit:\n\n")
	}
  // The following command emits an OSC8 code to embed clickable hyperlinks in
  // the terminal.
  // 
  // OSC8 codes are supported by gnome-terminal, hterm, iTerm, Terminal.  tmux version 3.20a
  // seems to support OSC8, but tmux 3.0 does not.  The OSC8 codes are (or seem to be) harmless
  // in terminals that don't support them.
  //
  // For more information: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
  fmt.Printf("\t\033]8;;%s\033\\%s\033]8;;\033\\\n", ares.Url, ares.Url);
	fmt.Printf("\nTo complete authentication with @%s.\n"+
		"I'll be waiting for you, but hurry! The request may timeout.\nHit Ctl+C with no regrets to abort.\n", domain)
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
	sshPub, sshPriv, err := kcerts.GenerateED25519()
	if err != nil {
		return nil, err
	}
	treq := &apb.TokenRequest{
		Url:       ares.Url,
		Publickey: ssh.MarshalAuthorizedKey(sshPub),
	}
	var tres *apb.TokenResponse
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
	return &EnkitCredentials{
		Token:          string(decrypted),
		CAPublicKey:    string(tres.Capublickey),
		CaHosts:        tres.Cahosts,
		PrivateKey:     sshPriv,
		SSHCertificate: cert,
	}, nil
}
