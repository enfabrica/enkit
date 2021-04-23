package auth

import (
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"github.com/enfabrica/enkit/lib/oauth/ogoogle"
)

func oauthModMap() map[string]oauth.Modifier {
	return map[string]oauth.Modifier{
		"google": ogoogle.Defaults(),
		"github": ogithub.Defaults(),
	}
}

// FetchCredentialOpts fetches credentials from a string type. If the string is empty or otherwise does not match
// it returns ogoogle.Defaults
func FetchCredentialOpts(t string) oauth.Modifier {
	if v, ok := oauthModMap()[t]; ok {
		return v
	}
	return ogoogle.Defaults()
}
