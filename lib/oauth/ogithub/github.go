package ogithub

import (
	"context"
	"fmt"

	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func Defaults() oauth.Modifier {
	return oauth.WithModifiers(
		oauth.WithEndpoint(github.Endpoint),
		oauth.WithFactory(NewGetUserVerifier),
	)
}

type GetUserVerifier struct {
	conf *oauth2.Config
}

func (guv *GetUserVerifier) Scopes() []string {
	return []string{
		"repos",
	}
}

func (guv *GetUserVerifier) Verify(log logger.Logger, identity *oauth.Identity, tok *oauth2.Token) (*oauth.Identity, error) {
	client := gh.NewClient(guv.conf.Client(oauth2.NoContext, tok))

	// FIXME: timeout, retry strategy.
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("retrieving user information failed - %w", err)
	}
	if user.ID == nil || user.Login == nil {
		return nil, fmt.Errorf("email and user ID not available - %w", err)
	}

	identity.Username = *user.Login
	identity.Organization = "github.com"
	identity.Id = fmt.Sprintf("github:%d", *user.ID)

	return identity, nil
}

// GetUserVerifier tries to fetch the userinfo of a user to verify the validity of a token.
//
// It performs an http request for every attempt to validate the token. If the request fails,
// either the token is invalid, or there is a problem with the API backend.
func NewGetUserVerifier(conf *oauth2.Config) (oauth.Verifier, error) {
	return &GetUserVerifier{conf: conf}, nil
}
