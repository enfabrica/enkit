package providers

import (
	"fmt"
	"regexp"
	"golang.org/x/oauth2"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/logger"
)

func NewGroupsKeeperFactory(keep, rename string) (oauth.VerifierFactory, error) {
	var keepr *regexp.Regexp
	var err error

	if keep != "" {
		keepr, err = regexp.Compile(keep)
		if err != nil {
			return nil, fmt.Errorf("could not compile keep regexp '%s': %w", keep, err)
		}
	}

	return func(conf *oauth2.Config) (oauth.Verifier, error) {
		return &GroupsKeeper{keep: keepr, rename: rename}, nil
	}, nil
}

type GroupsKeeper struct {
	keep *regexp.Regexp
	rename string
}

func (gk *GroupsKeeper) Scopes() []string {
	return nil
}

func (gk *GroupsKeeper) Verify(log logger.Logger, identity *oauth.Identity, tok *oauth2.Token) (*oauth.Identity, error) {
	newgroups := []string{}
	for _, group := range identity.Groups {
		if gk.keep != nil && !gk.keep.MatchString(group) {
			continue
		}

		if gk.rename != "" {
			group = gk.keep.ReplaceAllString(group, gk.rename)
		}

		newgroups = append(newgroups, group)
	}

	identity.Groups = newgroups
	return identity, nil
}
