package auth

import (
	"context"
	"crypto"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
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

	caSigner              crypto.Signer
	principals            []string
	marshalledCAPublicKey []byte
	userCertTTL           time.Duration
	log                   logger.Logger
}

func (s *Server) HostCertificate(ctx context.Context, request *auth.HostCertificateRequest) (*auth.HostCertificateResponse, error) {
	b, _ := pem.Decode(request.Hostcert)
	if b == nil {
		return nil, errors.New("the public key was empty, or was an invlaid block")
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(b.Bytes)
	if err != nil {
		return nil, err
	}

	cert, err := kcerts.SignPublicKey(s.caSigner, ssh.HostCert, request.Hosts, s.userCertTTL, pubKey)
	if err != nil {
		return nil, err
	}
	return &auth.HostCertificateResponse{
		Capublickey:    s.marshalledCAPublicKey,
		Signedhostcert: ssh.MarshalAuthorizedKey(cert),
	}, nil
}

type Jar struct {
	created time.Time
	channel chan oauth.AuthData
	cancel  context.CancelFunc
}

func (s *Server) GetChannel(cancel context.CancelFunc, pub common.Key) chan oauth.AuthData {
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
		channel: make(chan oauth.AuthData, 1),
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

func (s *Server) FeedToken(key common.Key, cookie oauth.AuthData) {
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

	case authData := <-channel:
		var nonce [common.NonceLength]byte
		if _, err := io.ReadFull(s.rng, nonce[:]); err != nil {
			return nil, status.Errorf(codes.Internal, "could not generate nonce - %s", err)
		}
		b, _ := pem.Decode(req.Publickey)
		// If the ca signer is nil that means the CA was never passed in flags, if the request never sent a public key
		// then so ssh certs will be sent back.
		if s.caSigner == nil || b == nil {
			return &auth.TokenResponse{
				Nonce: nonce[:],
				Token: box.Seal(nil, []byte(authData.Cookie), &nonce, (*[32]byte)(clientPub), (*[32]byte)(s.serverPriv)),
			}, nil
		}
		// If the ca signer was present, continuing with public keys.
		savedPubKey, _, _, _, err := ssh.ParseAuthorizedKey(b.Bytes)
		if err != nil {
			return nil, err
		}
		effectivePrincipals := append(s.principals, authData.Creds.Identity.Username)
		userCert, err := kcerts.SignPublicKey(s.caSigner, ssh.UserCert, effectivePrincipals, s.userCertTTL, savedPubKey)
		if err != nil {
			return nil, fmt.Errorf("error generating certificates: %w", err)
		}
		return &auth.TokenResponse{
			Nonce:       nonce[:],
			Token:       box.Seal(nil, []byte(authData.Cookie), &nonce, (*[32]byte)(clientPub), (*[32]byte)(s.serverPriv)),
			Capublickey: s.marshalledCAPublicKey,
			// Always trust the CA for now since the DNS gets resolved behind tunnel and therefore the client doesn't know
			// which to trust.
			Cahosts: []string{"*"},
			Cert:    ssh.MarshalAuthorizedKey(userCert),
		}, nil
	}

	return nil, status.Errorf(codes.Internal, "never reached, clearly")
}
