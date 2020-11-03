// Utility functions to load, process, and store identity tokens in a config store.
package identity

import (
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/kflags"
	"strings"
)

// IdentityFlags stores the values retrieved form the command line use to manage identities.
type IdentityFlags struct {
	// Full identity of the user. Indicates the identity of the human on behalf of which the operation
	// has to be completed. It generally looks like an email address. For example, "user@domain.com".
	UserDomain string

	// Identifies against whose infrastructure the operation has to be completed. It is generally a
	// domain name. For example, "enfabrica.net" indicates that the settings and servers of enfabrica.net
	// need to be used.
	DefaultDomain string
}

// DefaultIdentityFlags returns a default initialized IdentityFlags object.
func DefaultIdentityFlags() *IdentityFlags {
	return &IdentityFlags{}
}

// Register will register the flags necessary to configure the object.
func (ifl *IdentityFlags) Register(flags kflags.FlagSet, prefix string) *IdentityFlags {
	flags.StringVar(&ifl.UserDomain, prefix+"identity", "", "Default identity (eg, user@domain.com) to use to connect to the remote server")
	flags.StringVar(&ifl.DefaultDomain, prefix+"domain", "", "Default domain (eg, domain.com) to use to connect to the remote server")
	return ifl
}

// Identity of the user requesting the operation as specified via command line flags.
// It is generally a string that looks like an email, user@domain.com, for example.
//
// The empty string indicates the "default identity", unknown until loaded from a file or
// retrieved from some remote system.
func (ifl *IdentityFlags) Identity() string {
	return Join(SplitUsername(ifl.UserDomain, ifl.DefaultDomain))
}

// Same as Identity() but instead of returning an empty string as the default identity,
// it returns a human readable "(default)".
func (ifl *IdentityFlags) Printable() string {
	id := ifl.Identity()
	if id == "" {
		id = "(default)"
	}
	return id
}

// Domain name specified by the user via flags.
func (ifl *IdentityFlags) Domain() string {
	_, domain := SplitUsername(ifl.UserDomain, ifl.DefaultDomain)
	return domain
}

// User name specified by the user via flags.
func (ifl *IdentityFlags) User() string {
	user, _ := SplitUsername(ifl.UserDomain, ifl.DefaultDomain)
	return user
}

// SplitUsername will divide a name like user@domain.com into two parts: user, and domain.com.
// If no domain is specified in the username (eg, no @domain.com part), the defaultDomain
// is used instead.
func SplitUsername(username, defaultDomain string) (string, string) {
	ix := strings.LastIndex(username, "@")
	if ix < 0 {
		return username, defaultDomain
	}

	return username[:ix], username[ix+1:]
}

// Join will join a username and domain name into a user@domain address.
//
// If both are the empty string, it will also return the empty string, to
// represent the null identity.
func Join(username, domain string) string {
	if username == "" && domain == "" {
		return ""
	}

	return username + "@" + domain
}

// Token is the data structure that is serialized on disk to store a user token.
type Token struct {
	Token string
}

// Default is the data structure that is serialized on disk to store the default identity.
type Default struct {
	Identity string
}

// An Identity store is a generic object capable of storing and retrieving identities.
type IdentityStore interface {
	Save(identity string, token string) error
	SetDefault(identity string) error
	Load(identity string) (string, string, error)
}

// A ConfigIdentityStore is an IdentityStore using a config.Store to store and retrieve identities.
type ConfigIdentityStore struct {
	store config.Store
}

// NewStore returns a new Identity object.
//
// Identity objects are capable of loading and storing identities for the user.
//
// The configName parameter specifies the namespace used to store and retrieve
// those identities. The namespace is typically mapped to a config directory
// name where those identities are stored.
func NewStore(configName string, opener config.Opener) (*ConfigIdentityStore, error) {
	store, err := opener(configName, "identity")
	if err != nil {
		return nil, err
	}
	return &ConfigIdentityStore{store: store}, nil
}

func (id *ConfigIdentityStore) Save(identity string, token string) error {
	return id.store.Marshal(identity, Token{Token: token})
}
func (id *ConfigIdentityStore) SetDefault(identity string) error {
	return id.store.Marshal("default", Default{Identity: identity})
}

// Loads an identity from disk.
// identity is a string like 'user@domain.com', if empty, the default identity is loaded.
//
// Returns the identity loaded and the security token, or an error.
func (id *ConfigIdentityStore) Load(identity string) (string, string, error) {
	if identity == "" {
		var def Default
		if _, err := id.store.Unmarshal("default", &def); err != nil {
			return identity, "", err
		}
		identity = def.Identity
	}

	var token Token
	_, err := id.store.Unmarshal(identity, &token)
	return identity, token.Token, err
}
