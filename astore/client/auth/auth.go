package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/pkg/browser"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"math/rand"
	"time"
)

// Override the browser library defaults - just write to /dev/null, no need to
// print garbage on the console.
func init() {
	browser.Stdout = nil
	browser.Stderr = nil
}

type Client struct {
	rng  *rand.Rand
	conn grpc.ClientConnInterface
	auth auth.AuthClient
}

func New(rng *rand.Rand, conn grpc.ClientConnInterface) *Client {
	auth := auth.NewAuthClient(conn)
	return &Client{rng: rng, conn: conn, auth: auth}
}

type LoginOptions struct {
	*ccontext.Context

	// Minimum time that has to elapse between an attempt to retrieve
	// the token and the next. This is meant to prevent busy loops DoSsing
	// the server, while allowing fast retries in the normal case.
	MinWait time.Duration
	Store   cache.Store
}

func (c *Client) Login(username, domain string, o LoginOptions) (string, error) {
	pub, priv, err := box.GenerateKey(c.rng)
	if err != nil {
		return "", err
	}

	areq := &auth.AuthenticateRequest{
		Key:    (*pub)[:],
		User:   username,
		Domain: domain,
	}
	o.Logger.Infof("Retrieving authentication url.")
	ares, err := c.auth.Authenticate(context.TODO(), areq)
	if err != nil {
		return "", fmt.Errorf("Could not contact the authentication server. Is your connectivity working? Is the server up?\nFor debugging: %w", err)
	}

	servPub, err := common.KeyFromSlice(ares.Key)
	if err != nil {
		return "", fmt.Errorf("server provided invalid key - please retry - %s", err)
	}

	if username != "" {
		fmt.Printf("Dear %s, please visit:\n\n", username)
	} else {
		fmt.Printf("Kind human, please visit:\n\n")
	}
	fmt.Printf("\t%s\n\nTo complete authentication with @%s.\n"+
		"I'll be waiting for you, but hurry! The request may timeout.\nHit Ctl+C with no regrets to abort.\n", ares.Url, domain)
	browser.OpenURL(ares.Url)
	treq := &auth.TokenRequest{
		Url: ares.Url,
	}

	var tres *auth.TokenResponse
	for {
		o.Logger.Infof("Polling to retrieve token.")
		start := time.Now()
		tres, err = c.auth.Token(context.TODO(), treq)
		if err == nil {
			break
		}
		elapsed := time.Now().Sub(start)
		if elapsed < o.MinWait {
			in := o.MinWait - elapsed
			o.Logger.Infof("Polling failed - %v - retrying in %s", err, in)
			time.Sleep(o.MinWait - elapsed)
		} else {
			o.Logger.Infof("Polling failed - %v - retrying immediately", err)
		}
	}
	o.Logger.Infof("Polling succeeded - decrypting token")

	nonce, err := common.NonceFromSlice(tres.Nonce)
	if err != nil {
		return "", fmt.Errorf("server returned invalid nonce, please try again - %s", err)
	}

	decrypted, ok := box.Open(nil, tres.Token, nonce.ToByte(), servPub.ToByte(), priv)
	if !ok {
		return "", fmt.Errorf("could not decrypt returned token")
	}
	err = loadSSHKey(tres, o.Store, o.Logger)
	if err != nil {
		return "", err
	}
	return string(decrypted), err
}

func loadSSHKey(tres *auth.TokenResponse, store cache.Store, log logger.Logger) error {
	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(tres.Capublickey)
	if err != nil {
		return err
	}
	publicCertKey, _, _, _, err := ssh.ParseAuthorizedKey(tres.Cert)
	if err != nil {
		return err
	}
	r, _ := pem.Decode(tres.Key)
	privateKey, err := x509.ParsePKCS1PrivateKey(r.Bytes)
	if err != nil {
		return err
	}
	sshDir, err := kcerts.FindSSHDir()
	if err != nil {
		return err
	}
	err = kcerts.AddSSHCAToClient(caPublicKey, tres.Cahosts, sshDir)
	if err != nil {
		return err
	}
	agent, err := kcerts.FindSSHAgent(store, log)
	if err != nil {
		return err
	}
	defer agent.Close()
	err = agent.AddCertificates(privateKey, publicCertKey, uint32((time.Hour * 48).Seconds()))
	if err != nil {
		return err
	}
	log.Warnf("successfully added certificates to the ssh agent")
	return nil
}
