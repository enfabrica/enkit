package auth

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"math/rand"
	"sync"
	"time"
)

type Server struct {
	rng                   *rand.Rand
	serverPub, serverPriv *common.Key

	jarlock sync.Mutex
	jars    map[common.Key]*Jar

	authURL string
	limit   time.Duration

	caSigner              ssh.Signer
	principals            []string
	marshalledCAPublicKey []byte
	userCertTTL           time.Duration
}

type Jar struct {
	created time.Time
	channel chan string
	cancel  context.CancelFunc
}

func (s *Server) GetChannel(cancel context.CancelFunc, pub common.Key) chan string {
	s.jarlock.Lock()
	defer s.jarlock.Unlock()

	jar := s.jars[pub]
	if jar != nil {
		if jar.cancel != nil {
			jar.cancel()
		}
		jar.cancel = cancel
		return jar.channel
	}

	jar = &Jar{
		created: time.Now(),
		// Hold at least one token in the buffer.
		//
		// This allows a client supllying a token to not block until the token
		// has in facts been consumed.
		channel: make(chan string, 1),
		cancel:  cancel,
	}
	s.jars[pub] = jar
	return jar.channel
}

func (s *Server) Authenticate(ctx context.Context, req *auth.AuthenticateRequest) (*auth.AuthenticateResponse, error) {
	key, err := common.KeyFromSlice(req.Key)
	if err != nil {
		return nil, err
	}
	resp := &auth.AuthenticateResponse{
		Key: (*s.serverPub)[:],
		Url: fmt.Sprintf("%s/%s", s.authURL, hex.EncodeToString(key[:])),
	}
	return resp, nil
}

func (s *Server) FeedToken(key common.Key, cookie string) {
	channel := s.GetChannel(nil, key)
	channel <- cookie
}

func (s *Server) Token(ctx context.Context, req *auth.TokenRequest) (*auth.TokenResponse, error) {
	clientPub, err := common.KeyFromURL(req.Url)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	channel := s.GetChannel(cancel, *clientPub)
	select {
	case <-ctx.Done():
		return nil, status.Errorf(codes.Canceled, "context canceled while waiting for authentication")
	case <-time.After(s.limit):
		return nil, status.Errorf(codes.DeadlineExceeded, "timed out waiting for your lazy fingers to complete authentication")

	case token := <-channel:
		var nonce [common.NonceLength]byte
		if _, err := io.ReadFull(s.rng, nonce[:]); err != nil {
			return nil, status.Errorf(codes.Internal, "could not generate nonce - %s", err)
		}
		userPrivateKey, userCert, err := kcerts.GenerateUserSSHCert(s.caSigner, ssh.UserCert, s.principals, s.userCertTTL)
		if err != nil {
			return nil, fmt.Errorf("error generating certificates: %w", err)
		}
		return &auth.TokenResponse{
			Nonce:       nonce[:],
			Token:       box.Seal(nil, []byte(token), &nonce, (*[32]byte)(clientPub), (*[32]byte)(s.serverPriv)),
			Capublickey: s.marshalledCAPublicKey,
			// Always trust the CA for now since the DNS gets resolved behind tunnel and therefore the client doesn't know
			// which to trust
			Cahosts: []string{"*"},
			Cert:    ssh.MarshalAuthorizedKey(userCert),
			Key: pem.EncodeToMemory(&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(userPrivateKey),
			}),
		}, nil
	}

	return nil, status.Errorf(codes.Internal, "never reached, clearly")
}
