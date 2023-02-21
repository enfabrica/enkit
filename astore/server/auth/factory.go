package auth

import (
	"crypto/rsa"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/logger"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/enfabrica/enkit/astore/common"
	"github.com/enfabrica/enkit/lib/kflags"
	"golang.org/x/crypto/nacl/box"
)

type Flags struct {
	TimeLimit         time.Duration
	AuthURL           string
	Principals        string
	UseGroups	  bool
	CA                []byte
	UserCertTimeLimit time.Duration
}

func DefaultFlags() *Flags {
	return &Flags{
		TimeLimit: time.Minute * 30,
		UseGroups: true,
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.DurationVar(&f.TimeLimit, prefix+"time-limit", f.TimeLimit, "How long to wait at most for the user to complete authentication, before freeing resources")
	set.DurationVar(&f.UserCertTimeLimit, prefix+"user-cert-ttl", 24*time.Hour, "How long a user's ssh certificates are valid for before they expire")
	set.StringVar(&f.Principals, prefix+"principals", f.Principals, "Authorized ssh users which the ability to auth, in a comma separated string e.g. \"john,root,admin,smith\"")
	set.ByteFileVar(&f.CA, prefix+"ca", "", "Path to the certificate authority private file")
	set.BoolVar(&f.UseGroups, prefix+"use-groups", f.UseGroups, "If set to true, user groups are saved as principals in the user certificate")
	return f
}

func WithFlags(f *Flags) Modifier {
	return func(s *Server) error {
		if err := WithTimeLimit(f.TimeLimit)(s); err != nil {
			return err
		}
		if err := WithAuthURL(f.AuthURL)(s); err != nil {
			return err
		}
		if err := WithCA(f.CA)(s); err != nil {
			return err
		}
		if err := WithUserCertTimeLimit(f.UserCertTimeLimit)(s); err != nil {
			return err
		}
		if err := WithPrincipals(f.Principals)(s); err != nil {
			return err
		}
		if err := WithUseGroups(f.UseGroups)(s); err != nil {
			return err
		}
		if s.authURL == "" || s.authURL == "/" {
			return fmt.Errorf("an auth-url must be supplied using the --auth-url parameter")
		}
		return nil
	}
}

type Modifier func(*Server) error

// WithAuthURL supplies the URL to send users to for authentication.
// It is the URL of a running astore server.
func WithAuthURL(url string) Modifier {
	return func(s *Server) error {
		s.authURL = url
		return nil
	}
}

// WithUseGroups enables (or disables) the propagation of user groups as principals.
func WithUseGroups(use bool) Modifier {
	return func(s *Server) error {
		s.useGroups = use
		return nil
	}
}

func WithTimeLimit(limit time.Duration) Modifier {
	return func(s *Server) error {
		s.limit = limit
		return nil
	}
}

func WithCA(fileContent []byte) Modifier {
	return func(server *Server) error {
		if len(fileContent) == 0 {
			server.log.Warnf("CA file not specified - will operate without certificates")
			return nil
		}
		caPrivateKey, err := ssh.ParseRawPrivateKey(fileContent)
		if err != nil {
			return fmt.Errorf("Could not parse CA key - %w", err)
		}
		// TODO(adam): make parsing existing keys cleaner
		if key, ok := caPrivateKey.(*ed25519.PrivateKey); ok {
			server.caPrivateKey = kcerts.FromEC25519(*key)
			sshPubKey, err := ssh.NewPublicKey(key.Public())
			if err != nil {
				return err
			}
			server.marshalledCAPublicKey = ssh.MarshalAuthorizedKey(sshPubKey)
			return nil
		}
		if key, ok := caPrivateKey.(*rsa.PrivateKey); ok {
			server.caPrivateKey = kcerts.FromRSA(key)
			sshPubKey, err := ssh.NewPublicKey(key.Public())
			if err != nil {
				return err
			}
			server.marshalledCAPublicKey = ssh.MarshalAuthorizedKey(sshPubKey)
			return nil
		}
		return fmt.Errorf("keys could not be processed, keys of type %v are not supported", reflect.TypeOf(caPrivateKey))
	}
}

func WithPrincipals(raw string) Modifier {
	return func(server *Server) error {
		splitString := strings.Split(raw, ",")
		if len(splitString) >= 1 && splitString[0] != "" {
			server.principals = splitString
		}
		return nil
	}
}

func WithUserCertTimeLimit(duration time.Duration) Modifier {
	return func(server *Server) error {
		server.userCertTTL = duration
		return nil
	}
}

func WithLogger(log logger.Logger) Modifier {
	return func(server *Server) error {
		server.log = log
		return nil
	}
}

func New(rng *rand.Rand, mods ...Modifier) (*Server, error) {
	pub, priv, err := box.GenerateKey(rng)
	if err != nil {
		return nil, err
	}

	s := &Server{
		rng:        rng,
		serverPub:  (*common.Key)(pub),
		serverPriv: (*common.Key)(priv),
		useGroups:  true,
		jars:       map[common.Key]*Jar{},
		limit:      30 * time.Minute,
	}

	for _, m := range mods {
		if err := m(s); err != nil {
			return nil, err
		}
	}

	s.authURL = strings.TrimSuffix(s.authURL, "/")
	if s.authURL == "" {
		return nil, fmt.Errorf("API usage error - an authentication URL must be set")
	}

	return s, nil
}
