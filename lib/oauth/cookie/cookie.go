// Collection of utilities to more easily compose cookies.
package cookie

import (
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"net/http"
)

// CredentialsCookieName returns the name of the cookie maintaing the set of user credentials.
//
// This cookie is the one used to determine what the user can and cannot do on the UI.
func CredentialsCookieName(prefix string) string {
	return prefix + "Creds"
}

func CredentialsCookie(prefix, value string, co ...kcookie.Modifier) *http.Cookie {
	return kcookie.New(CredentialsCookieName(prefix), value, co...)
}
