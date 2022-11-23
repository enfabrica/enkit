package oauth

import (
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"net/http"
)

// An IAuthenticator is any object capable of performing authentication for a web server.
type IAuthenticator interface {
	// PerformLogin initiates the login process.
	//
	// PerformLogin will redirect the user to the oauth IdP login page, after
	// generating encypted cookies containing enough information to verify success
	// at the end of the process and to carry application state.
	//
	// PerformLogin will initiate the process even if the user is already logged in.
	//
	// The PerformLogin may not support all the login modifiers. Specifically,
	// WithCookieOptions may be silently ignored if no cookie is used by the
	// specific implementation. If state is supplied with WithState and the
	// underlying implementation cannot propagate state, the error
	// ErrorStateUnsupported will be returned instead.
	PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error

	// PerformAuth turns the credentials received into AuthData.
	//
	// PerformAuth is invoked at the END of the authentication process. The URL
	// of the code invoking PerformAuth is typically configured as the oauth
	// endpoint.
	//
	// If no error is returned, AuthData is guaranteed to be usable, although
	// the Complete() method in AuthData can be used to verify that the process
	// returned valid credentials.
	//
	// If the error returned is ErrorNotAuthenticated, it means that
	// authentication data was not found at all, meaning that a Login process
	// probably needs to be started. This is useful to create handlers that
	// can act both as Login and Auth handlers, or to write handlers that
	// conditionally start the login process.
	PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, error)

	// GetCredentialsFromRequest extracts the credentials from an http request.
	//
	// This is useful to check if - for example - a user already authenticated
	// before invoking PerformLogin, or to verify that a credential cookie has
	// been supplied in a gRPC or headless application.
	//
	// If no authentication cookie is found (eg, user has not ever attempted
	// login), ErrorNotAuthenticated is returned. In general, though, if an
	// error is returned by GetCredentialsFromRequest the caller of this API
	// should invoke PerformLogin to re-try the login process blindly.
        GetCredentialsFromRequest(r *http.Request) (*CredentialsCookie, string, error)
}
