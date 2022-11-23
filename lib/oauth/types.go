package oauth

import (
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"net/http"
)

// An IAuthenticator is any object capable of performing authentication for a web server.
// PerformLogin initiates the login process.
// PerformAuth is invoked at the end, to verify that the login was successful.
type IAuthenticator interface {
	PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error
	PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, error)
}
