package auth

import (
	"context"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/nacl/box"
	"math/rand"
	"strings"
	"testing"
)

func TestInvalid(t *testing.T) {
	rng := rand.New(srand.Source)
	server, err := New(rng)
	assert.Nil(t, server)
	assert.NotNil(t, err)
}

func Authenticate(t *testing.T, rng *rand.Rand, server *Server) *auth.TokenResponse {
	pub, priv, err := box.GenerateKey(rng)
	assert.Nil(t, err, err)

	areq := &auth.AuthenticateRequest{
		Key:    (*pub)[:],
		User:   "emma.goldman",
		Domain: "writers.org",
	}

	aresp, err := server.Authenticate(context.Background(), areq)
	assert.Nil(t, err, err)
	assert.Equal(t, 32, len(aresp.Key), "%d", len(aresp.Key))
	assert.True(t, strings.HasPrefix(aresp.Url, "static-prefix"), aresp.Url)
	servPub, err := common.KeyFromSlice(aresp.Key)
	assert.Nil(t, err, err)

	key, err := common.KeyFromURL(aresp.Url)
	assert.Nil(t, err, err)
	assert.NotNil(t, key)

	violence := "The most violent element in society is ignorance."
	oa := oauth.AuthData{Creds: &oauth.CredentialsCookie{Identity: oauth.Identity{
		Id:           "emma.goldman@writers.org",
		Username:     "emma.goldman",
		Organization: "writers.org",
	}}, Cookie: violence}
	server.FeedToken(*key, oa)

	treq := &auth.TokenRequest{
		Url: aresp.Url,
	}
	tresp, err := server.Token(context.Background(), treq)
	assert.Nil(t, err, err)
	assert.NotNil(t, tresp)

	assert.Equal(t, 65, len(tresp.Token), "%v", tresp.Token)
	assert.Equal(t, 24, len(tresp.Nonce), "%v", tresp.Nonce)

	nonce, err := common.NonceFromSlice(tresp.Nonce)
	decrypted, ok := box.Open(nil, tresp.Token, nonce.ToByte(), servPub.ToByte(), priv)
	assert.True(t, ok)

	assert.Equal(t, violence, string(decrypted), "%v - %v", decrypted, string(decrypted))
	return tresp
}

func TestBasicAuth(t *testing.T) {
	rng := rand.New(srand.Source)
	server, err := New(rng, WithAuthURL("static-prefix"))
	assert.Nil(t, err, err)
	assert.NotNil(t, server)

	tresp := Authenticate(t, rng, server)
	assert.Equal(t, 0, len(tresp.Key), "%v", tresp.Key)
	assert.Equal(t, 0, len(tresp.Cert), "%v", tresp.Cert)
	assert.Equal(t, 0, len(tresp.Capublickey), "%v", tresp.Capublickey)
}

// Just in case your security scanner goes crazy on this:
// This is a test key, it is actually not used anywehere, at all.
// Yes, it has no passphrase.
//
// Feel free to install the corresponding public key on your servers, and enjoy
// all visistors who scan github repositories for private keys.
var rsaTestKey = `-----BEGIN OPENSSH PRIVATE KEY-----
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
-----END OPENSSH PRIVATE KEY-----`

func TestCAAuthBroken(t *testing.T) {
	rng := rand.New(srand.Source)

	whacky := []byte("I don't need no certificate")
	server, err := New(rng, WithAuthURL("static-prefix"), WithCA(whacky))
	assert.NotNil(t, err, err)
	assert.Nil(t, server)
}

func TestCAAuthRSA(t *testing.T) {
	rng := rand.New(srand.Source)

	server, err := New(rng, WithAuthURL("static-prefix"), WithCA([]byte(rsaTestKey)))
	assert.Nil(t, err, err)
	assert.NotNil(t, server)

	tresp := Authenticate(t, rng, server)
	assert.Less(t, 128, len(tresp.Key), "%v", tresp.Key)
	assert.Less(t, 128, len(tresp.Cert), "%v", tresp.Cert)
	assert.Less(t, 128, len(tresp.Capublickey), "%v", tresp.Capublickey)
}

// This is actually the real private key that gives you access to
// all of the NASA infrastructure, go use it.
//
// (joking of course, if your security scanner goes crazy on this,
// go read the comment a few paragraphs above).
var edTestKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDjOVZs0VjbcZ+1Bui+OlOoLxn57G7pqk6CdwEQxTQLxwAAAJhzXgcpc14H
KQAAAAtzc2gtZWQyNTUxOQAAACDjOVZs0VjbcZ+1Bui+OlOoLxn57G7pqk6CdwEQxTQLxw
AAAECpMUD96P39OuqM0tL8NI5nw30BZGm1Du7ILZSz/Sjv+eM5VmzRWNtxn7UG6L46U6gv
GfnsbumqToJ3ARDFNAvHAAAAEWNjb250YXZhbGxpQG5vcmFkAQIDBA==
-----END OPENSSH PRIVATE KEY-----`

func TestCAAuthED25519(t *testing.T) {
	rng := rand.New(srand.Source)

	server, err := New(rng, WithAuthURL("static-prefix"), WithCA([]byte(edTestKey)))
	assert.Nil(t, err, err)
	assert.NotNil(t, server)

	tresp := Authenticate(t, rng, server)

	// ed25519 keys are significantly smaller.
	assert.Less(t, 80, len(tresp.Key), tresp.Key)
	assert.Less(t, 80, len(tresp.Cert), tresp.Cert)
	assert.Less(t, 80, len(tresp.Capublickey), tresp.Capublickey)
}
