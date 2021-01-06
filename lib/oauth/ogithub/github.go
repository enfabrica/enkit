package ogithub

import (
	"context"
	"fmt"
	"log"

	"github.com/enfabrica/enkit/lib/oauth"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func Defaults() oauth.Modifier {
	return oauth.WithModifiers(
		oauth.WithScopes([]string{
			"repos",
		}),
		oauth.WithEndpoint(github.Endpoint),
		oauth.WithFactory(GetUserVerifier),
	)
}

// GetUserVerifier tries to fetch the userinfo of a user to verify the validity of a token.
//
// It performs an http request for every attempt to validate the token. If the request fails,
// either the token is invalid, or there is a problem with the API backend.
func GetUserVerifier(conf *oauth2.Config) (oauth.Verifier, error) {
	return func(tok *oauth2.Token) (*oauth.Identity, error) {
		client := gh.NewClient(conf.Client(oauth2.NoContext, tok))
		log.Printf("TOK - %#v", tok)

		// FIXME: timeout, retry strategy.
		user, _, err := client.Users.Get(context.Background(), "")
		if err != nil {
			return nil, fmt.Errorf("retrieving user information failed - %w", err)
		}
		if user.ID == nil || user.Login == nil {
			return nil, fmt.Errorf("email and user ID not available - %w", err)
		}

		log.Printf("USER - %#v", user)

		return &oauth.Identity{
			Id:           fmt.Sprintf("github:%d", *user.ID),
			Username:     *user.Login,
			Organization: "github.com",
		}, nil
	}, nil
}
