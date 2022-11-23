package oauth

import (
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"net/http"
)

// An IAuthenticator is any object capable of performing authentication for a web server.
//
// PerformLogin initiates the login process, no matter if the user is already
// logged in or not.
// The PerformLogin may not support all the login modifiers. Specifically,
// WithCookieOptions may be silently ignored if no cookie is used by the
// specific implementation. If state is supplied with WithState and the
// underlying implementation cannot propagate state, the error
// ErrorStateUnsupported will be returned instead.
//
// PerformAuth is invoked at the end of the authentication process, to turn
// the credentials received into a uniform data structure, AuthData.
// If the error returned is ErrorNotAuthenticated, it means that
// authentication data was not found at all, meaning that a Login process
// probably needs to be started. This is useful to create handlers that
// can act both as Login and Auth handlers, or to write handlers that
// conditionally start the login process.
type IAuthenticator interface {
	PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error
	PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, error)
}
