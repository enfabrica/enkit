// Utility functions to load, process, and store identity tokens in a config store.
//
//
package identity

import (
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/kflags"
	"strings"
)

type Flags string

func DefaultFlags() *Flags {
	def := ""
	return (*Flags)(&def)
}

func (ifl *Flags) Register(flags kflags.FlagSet, prefix string) *Flags {
	flags.StringVar((*string)(ifl), prefix+"identity", "", "Default identity to use to connect to the remote server")
	return ifl
}
func (ifl Flags) Identity() string {
	return (string)(ifl)
}

func SplitUsername(username, defaultDomain string) (string, string) {
	ix := strings.LastIndex(username, "@")
	if ix < 0 {
		return username, defaultDomain
	}

	return username[:ix], username[ix+1:]
}

func Join(username, domain string) string {
	return username + "@" + domain
}

type Token struct {
	Token string
}

type Default struct {
	Identity string
}

type Identity struct {
	store config.Store
}

func NewStore(opener config.Opener) (*Identity, error) {
	store, err := opener("asuite", "identity")
	if err != nil {
		return nil, err
	}
	return &Identity{store: store}, nil
}

func (id *Identity) Save(identity string, token string) error {
	return id.store.Marshal(identity, Token{Token: token})
}
func (id *Identity) SetDefault(identity string) error {
	return id.store.Marshal("default", Default{Identity: identity})
}
func (id *Identity) Load(identity string) (string, string, error) {
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
