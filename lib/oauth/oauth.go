package oauth

// Simplifies the use of oauth2 with a net.http handler and grpc.
//
// To use the library, you have to:
//
//   1) Setup the oauth2 hanlders so the user can log in.
//   2) Verify the validity of the user credentials when performing privileged operations.
//
// Simple setup:
//
//     authenticator, err := New(..., WithSecrets(...), WithTarget("https://localhos:5433/auth"), ogoogle.Defaults())
//
//     [...]
//
//     http.HandleFunc("/auth", authenticator.AuthHandler())     // /auth will be the endpoint for oauth, store the cookie.
//     http.HandleFunc("/login", authenticator.LoginHandler())   // visiting /login will redirect to the oauth provider.
//
// More complex setup:
//
//     authenticator, err := New(..., WithSecrets(...), WithTarget("https://localhos:5433/auth"), ogoogle.Defaults())
//
//     [...]
//
//     http.HandleFunc("/", authenticator.MakeAuthHandler(authenticator.MakeLoginHandler(rootHandler, "")))
//
// Request authentication:
//
//    http.HandleFunc("/", authenticator.WithCredentials(rootHandler))
//
// or:
//
//    http.HandleFunc("/", authenticator.WithCredentialsOrRedirect(rootHandler, "/login"))
//
// From within your handler, you can use:
//
//    [...]
//    credentials := oauth.GetCredentials(r.Context())
//    if credentials == nil {
//        http.Error(w, "not authenticated", http.StatusInternalServerError)
//    } else {
//        log.Printf("email: %s", credentials.Identity.Email)
//    }
//
//

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/kassets"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/oauth/cookie"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/token"
)

// Verifier is an object capable of verifying an oauth2.Token after obtaining it.
//
// Verifiers can also add information retrieved from the remote provider to the
// identity, using some provider specific mechanisms.
//
// For example, they can check if a domain matches a list of allowed domains, or
// retrieve a list of groups and add them as part of the user identity.
type Verifier interface {
	Scopes() []string
	Verify(log logger.Logger, identity *Identity, tok *oauth2.Token) (*Identity, error)
}

type VerifierFactory func(conf *oauth2.Config) (Verifier, error)

type OptionalVerifier struct {
	inner Verifier
}

func (ov *OptionalVerifier) Scopes() []string {
	return ov.inner.Scopes()
}

func (ov *OptionalVerifier) Verify(log logger.Logger, identity *Identity, tok *oauth2.Token) (*Identity, error) {
	result, err := ov.inner.Verify(log, identity, tok)
	if err != nil {
		user := "<unknown>"
		if identity != nil {
			user = identity.GlobalName()
		}

		log.Warnf("for user %s - ignored verifier %T - error: %s", user, ov.inner, err)
		return identity, nil
	}
	return result, nil
}

func NewOptionalVerifierFactory(factory VerifierFactory) VerifierFactory {
	return func(conf *oauth2.Config) (Verifier, error) {
		inner, err := factory(conf)
		if err != nil {
			return nil, err
		}
		return &OptionalVerifier{inner: inner}, nil
	}
}

// Extractor is an object capable of extracting and verifying authentication information.
type Extractor struct {
	version int

	// Two versions of token.
	loginEncoder0 *token.TypeEncoder
	loginEncoder1 *token.TypeEncoder

	// String to prepend to the cookie name.
	// This is necessary when multiple instances of the oauth library are used within
	// the same application, or to ensure the uniqueness of the cookie name in a complex app.
	baseCookie string
}

// Redirector is an extractor capable of redirecting to an authentication server for login.
type Redirector struct {
	*Extractor
	AuthURL *url.URL
}

var ErrorLoops = errors.New("You have been redirected back to this url - but you still don't have an authentication token.\n" +
	"As a sentinent web server, I've decided that you human don't deserve any further redirect, as that would cause a loop\n" +
	"which would be bad for the future of the internet, my load, and your bandwidth. Hit refresh if you want, but there's likely\n" +
	"something wrong in your cookies, or your setup")
var ErrorCannotAuthenticate = errors.New("Who are you? Sorry, you have no authentication cookie, and there is no authentication service configured")

// Authenticate parses the request received to authenticate the user.
//
// Typically, the Authenticate() function will check if the request contains a valid
// cookie, some authentication header, or parameters passed in the URL. If everything
// goes well, it returns a CredentialsCookie. Otherwise, it will either start the
// authentication process (nil error is returned), or returns an error.
//
// There are thus 3 possible combinations of return values:
// - A non-nil CredentialsCookie, and a nil error - meaning that the request was
//   successfully authenticated. The CredentialsCookie contains details of the
//   user.
// - A nil CredentialsCookie, and a non-nil error - meaning that the request
//   was not authenticated, and that it was not possible to kickstart the
//   authentication process. The error is generally non-recoverable.
// - A nil CredentialsCookie, and a nil error - meaning that the request was
//   not authenticated, but that the authentication process was kickstarted.
//
//   In this case, the caller should consider the request already handled,
//   and just return. The Authenticate function either performed a redirect,
//   or otherwise took care of the next step (by, for example, setting a
//   cookie or displaying an error page).
//
// An Authenticate function should always do its best to try to authenticate
// the user, no matter what the request contains. An invalid or expired cookie
// or invalid headers should not result in an error returned. Instead, the
// function should attempt to re-authenticate the user.
//
// An Authenticate function should return an error only if there is some
// unrecoverable condition. For example, a server configuration error, an error
// in a crypto function, or a database error.
//
// Parameters are:
// - An http.ResponseWriter, used to perform redirects, or otherwise add cookies
//   or headers.
// - An http.Request, parsed to look for credentials.
// - A redirect url, a page to send the user back to in case authentication
//   is required, and the user has to be sent to a different page.
type Authenticate func(w http.ResponseWriter, r *http.Request, rurl *url.URL) (*CredentialsCookie, error)

func CreateRedirectURL(r *http.Request) *url.URL {
	rurl := khttp.RequestURL(r)
	rurl.RawQuery = khttp.JoinURLQuery(rurl.RawQuery, "_redirected")
	return rurl
}

func (as *Redirector) Authenticate(w http.ResponseWriter, r *http.Request, rurl *url.URL) (*CredentialsCookie, error) {
	creds, err := as.GetCredentialsFromRequest(r)
	if creds != nil && err == nil {
		return creds, nil
	}

	if as.AuthURL == nil {
		return nil, ErrorCannotAuthenticate
	}

	_, redirected := r.URL.Query()["_redirected"]
	if redirected {
		return nil, ErrorLoops
	}

	target := *as.AuthURL
	if rurl != nil {
		target.RawQuery = khttp.JoinURLQuery(target.RawQuery, "r="+url.QueryEscape(rurl.String()))
	}
	http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
	return nil, nil
}

type Authenticator struct {
	Extractor

	rng         *rand.Rand
	log	    logger.Logger
	authEncoder *token.TypeEncoder

	conf *oauth2.Config

	verifiers []Verifier
}

type Identity struct {
	// Id is a globally unique identifier of the user.
	// It is oauth provider specific, generally contains an integer or string
	// uniquely identifying the user, and a domain name used to namespace the id.
	Id           string
	// Username is the name of the user on the remote system.
	Username     string
	// Organization is the domain name used to authenticate the user.
	// For example, github.com, or the specific gsuite domain.
	Organization string
	// Groups is a list of string identifying the groups the user is part of.
	Groups       []string
}

// GlobalName returns a human friendly string identifying the user.
//
// It looks like an email, but it may or may not be a valid email address.
//
// For example: github users will have github.com as organization, and their login as Username.
//              The GlobalName will be username@github.com. Not a valid email.
// On the other hand: gsuite users for enfabrica.net will have enfabrica.net as organization,
//		and their username as Username, forming a valid email.
//
// Interpret the result as meaning "user by this name" @ "organization by this name".
func (i *Identity) GlobalName() string {
	return i.Username + "@" + i.Organization
}

// Based on the kind of identity obtained, returns a modifier able to generate
// certificates to support that specific identity type.
func (i *Identity) CertMod() kcerts.CertMod {
	if i.Organization == "github.com" {
		return func(certificate *ssh.Certificate) *ssh.Certificate {
			certificate.Extensions["login@github.com"] = i.Username
			return certificate
		}
	}
	return kcerts.NoOp
}

// CredentialsCookie is what is encrypted within the authentication cookie returned
// to the browser or client.
type CredentialsCookie struct {
	// An abstract representation of the identity of the user.
	// This is independent of the authentication provider.
	Identity Identity
	Token    oauth2.Token
}

// LoginURL computes the URL the user is redirected to to perform login.
//
// After the user authenticates, it is redirected back to URL set as auth handler,
// which verifies the credentials, and creates the authentication cookie.
//
// At this point, either the auth handler returns a page directly (for example, when
// you set up your own handler with MakeAuthHandler), or, if a target parameter is
// set, the user is redirected to the configured target.
//
// State is not used by the auth handler. You can basically pass anything you like
// and have it forwarded to you at the end of the authentication.
//
// Returns: the url to use, a secure token, and nil or an error, in order.
func (a *Authenticator) LoginURL(target string, state interface{}) (string, []byte, error) {
	secret := make([]byte, 16)
	_, err := a.rng.Read(secret)
	if err != nil {
		return "", nil, err
	}
	// This is not necessary. We could just pass the secret to the AuthCodeURL function.
	// But it needs to be escaped. AuthoCookie.Encode will sign it, as well as Encode it. Cannot hurt.
	esecret, err := a.authEncoder.Encode(LoginState{Secret: secret, Target: target, State: state})
	if err != nil {
		return "", nil, err
	}
	url := a.conf.AuthCodeURL(string(esecret))
	///* oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "login"), oauth2.SetAuthURLParam("approval_prompt", "force"), oauth2.SetAuthURLParam("max_age", "0") */)
	return url, secret, nil
}

// Mapper configures all the URLs to redirect to / unless an authentication cookie is provided by the browser.
// Further, it configures / to redirect and perform oauth authentication.
func (a *Authenticator) Mapper(mapper kassets.AssetMapper, lm ...LoginModifier) kassets.AssetMapper {
	return func(original, name string, handler khttp.FuncHandler) []string {
		ext := filepath.Ext(original)
		switch {
		case name == "/favicon.ico":
			return mapper(original, name, handler)
		case name == "/":
			return mapper(original, name, a.MakeAuthHandler(a.MakeLoginHandler(handler, lm...)))
		case ext == ".html":
			return mapper(original, name, a.WithCredentialsOrRedirect(handler, "/"))
		default:
			return mapper(original, name, a.WithCredentialsOrError(handler))
		}
	}
}

// GetCredentials returns the credentials of a user extracted from an authentication cookie.
// Returns nil if the context has no credentials.
func GetCredentials(ctx context.Context) *CredentialsCookie {
	creds, _ := ctx.Value("creds").(*CredentialsCookie)
	return creds
}

// SetCredentials returns a context with the credentials of the user added.
// Use GetCredentials to retrieve them later.
func SetCredentials(ctx context.Context, creds *CredentialsCookie) context.Context {
	return context.WithValue(ctx, "creds", creds)
}

type credentialsKey string

var CredentialsVersionKey = credentialsKey("version")

type CredentialsMeta struct {
	context.Context
}

func (ctx CredentialsMeta) Issued() time.Time {
	issued, _ := ctx.Value(token.IssuedTimeKey).(time.Time)
	return issued
}

func (ctx CredentialsMeta) Expires() time.Time {
	expire, _ := ctx.Value(token.ExpiresTimeKey).(time.Time)
	return expire
}

func (ctx CredentialsMeta) Max() time.Time {
	max, _ := ctx.Value(token.MaxTimeKey).(time.Time)
	return max
}

func (ctx CredentialsMeta) Version() int {
	version, _ := ctx.Value(CredentialsVersionKey).(int)
	return version
}

// ParseCredentialsCookie parses a string containing a CredentialsCookie, and returns the corresponding object.
func (a *Extractor) ParseCredentialsCookie(cookie string) (CredentialsMeta, *CredentialsCookie, error) {
	var credentials CredentialsCookie
	var err error
	var ctx context.Context

	if strings.HasPrefix(cookie, "1:") {
		ctx, err = a.loginEncoder1.Decode(context.Background(), []byte(cookie[2:]), &credentials)
		ctx = context.WithValue(ctx, CredentialsVersionKey, 1)
	} else {
		ctx, err = a.loginEncoder0.Decode(context.Background(), []byte(cookie), &credentials)
	}
	return CredentialsMeta{ctx}, &credentials, err
}

// EncodeCredentials generates a string containing a CredentialsCookie.
func (a *Extractor) EncodeCredentials(creds CredentialsCookie) (string, error) {
	var result []byte
	var cookie string
	var err error
	switch a.version {
	case 0:
		result, err = a.loginEncoder0.Encode(creds)
		cookie = string(result)
	case 1:
		result, err = a.loginEncoder1.Encode(creds)
		cookie = "1:" + string(result)
	default:
		err = fmt.Errorf("invalid version %d", a.version)
	}
	if err != nil {
		return "", err
	}
	return cookie, nil
}

// GetCredentialsFromRequest will parse and validate the credentials in an http request.
//
// If successful, it will return a CredentialsCookie pointer.
// If no credentials, or invalid credentials, an error is returned with nil credentials.
func (a *Extractor) GetCredentialsFromRequest(r *http.Request) (*CredentialsCookie, error) {
	cookie, err := r.Cookie(a.CredentialsCookieName())
	if err != nil {
		return nil, err
	}

	_, credentials, err := a.ParseCredentialsCookie(cookie.Value)
	if err != nil {
		return nil, err
	}
	if credentials == nil {
		return nil, fmt.Errorf("invalid nil credentials")
	}
	return credentials, nil
}

// WithCredentials invokes the handler with the identity of the user supplied in the context.
//
// If the credentials are invalid or not avaialable, no identity is set in the context.
// Use credentials := GetCredentials(request.Context()) to access the information.
// If nil, the call is not authenticated.
//
// Normally, you should use WithCredentialsOrRedirect(). Use this function only if you
// expect your handler to be invoked with or without credentials.
func (a *Extractor) WithCredentials(handler khttp.FuncHandler) khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, err := a.GetCredentialsFromRequest(r)
		if creds != nil && err == nil {
			r = r.WithContext(SetCredentials(r.Context(), creds))
		}
		handler(w, r)
	}
}

// WithCredentialsOrRedirect invokes the handler if credentials are available, or redirects if they are not.
//
// Same as WithCredentials, except that invalid credentials result in a redirect to the specified target.
// GetCredentials() invoked from the handler is guaranteed to return a non null result.
func (a *Authenticator) WithCredentialsOrRedirect(handler khttp.FuncHandler, target string) khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, err := a.GetCredentialsFromRequest(r)
		if creds == nil || err != nil {
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		} else {
			r = r.WithContext(SetCredentials(r.Context(), creds))
			handler(w, r)
		}
	}
}

// WithCredentialsOrError invokes the handler if credentials are available, errors out if not.
func (a *Authenticator) WithCredentialsOrError(handler khttp.FuncHandler) khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		creds, err := a.GetCredentialsFromRequest(r)
		if creds == nil || err != nil {
			http.Error(w, "not authorized", http.StatusUnauthorized)
		} else {
			r = r.WithContext(SetCredentials(r.Context(), creds))
			handler(w, r)
		}
	}
}

// MakeLoginHandler turns the specified handler into a LoginHandler.
//
// LoginHandler (below) returns an http handler that always redirects the user to the login
// page of the configured provider.
//
// MakeLoginHandler (here) returns an http handler that will first check if the
// user is authenticated already.
//
// If authenticated, your handler will be invoked with the credentials of the user parsed
// in the context.
//
// If not authenticated, the user will be redirected to the login page. target is interpreted
// as the LoginHandler function describes. You should set it to ensure that the user is
// redirected back to this page after login.
//
// It is not computed automatically to avoid the nuisances of proxies or load
// balancers, having http vs https (scheme is not propagated), ... Just set it explicitly
// with your own code, ensuring it is an absolute URL.
//
// Note that login handlers need to be registered with your oauth provider.
func (a *Authenticator) MakeLoginHandler(handler khttp.FuncHandler, lm ...LoginModifier) khttp.FuncHandler {
	loginHandler := a.LoginHandler(lm...)

	return func(w http.ResponseWriter, r *http.Request) {
		creds := GetCredentials(r.Context())
		if creds != nil {
			r = r.WithContext(SetCredentials(r.Context(), creds))
			handler(w, r)
			return
		}

		creds, err := a.GetCredentialsFromRequest(r)
		if creds != nil && err == nil {
			r = r.WithContext(SetCredentials(r.Context(), creds))
			handler(w, r)
			return
		}
		loginHandler(w, r)
	}
}

// Creates and returns a LoginHandler.
//
// The LoginHandler is responsible for redirecting the user to the login page used by
// the oauth provider, while encoding all the parameters necessary to redirect the user
// back to this web site.
//
// The target string is which URL to redirect the user to at the end of authentication
// process and is optional, can be the empty string.
//
// Basically:
//   - LoginHandler -> redirects the user to google/github/... oauth login page.
//   - user successfuly logs in -> he is redirected to the page configured WithTarget().
//     This page must be an AuthHandler, as it needs to check the values returned by
//     the oauth provider.
//   - If a target url was set, the AuthHandler will issue a redirect to that URL.
//
// The target URL is necessary as most oauth providers have a limit to the number of
// pages that can be used as AuthHandler, no wildcards are supported, and the page
// must be configured with the oauth provider.
//
// But an authentication cookie can expire anywhere on your site, and you will need the
// user to be redirected where he was at the end of the authentication.
//
// Note that this call does not allow you to carry any additional state.
// Use session cookies for that part instead, or get parameters.
//
func (a *Authenticator) LoginHandler(lm ...LoginModifier) khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		err := a.PerformLogin(w, r, lm...)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Printf("ERROR - could not complete login - %s", err)
		}
	}
}

// MakeAuthHandler turns the specified handler into an AuthHandler.
//
// AuthHandler (below) returns an http handler that verifies the token returned by the
// oauth provider, and redirects to the target passed to the LoginHandler (if configured).
//
// MakeAuthHandler (here) returns an http handler that will first check if the request
// contains the information from the oauth provider.
//
// If yes, and the data is valid, it will process the authentication request, and if
// a target was passed, perform the redirect.
//
// If no, an error happens, or no redirect is performed, your handler is invoked.
//
// Note that auth handlers need to be registered with your oauth provider.
//
func (a *Authenticator) MakeAuthHandler(handler khttp.FuncHandler) khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := a.PerformAuth(w, r)
		if err == nil && data.Creds != nil {
			ctx := SetCredentials(r.Context(), data.Creds)
			r = r.WithContext(ctx)
		}
		if !CheckRedirect(w, r, data) {
			handler(w, r)
		}
	}
}

// AuthHandler returns the http handler to be invoked at the end of the oauth process.
//
// With oauth, an un-authenticated user will be first redirected to the login page
// of the oauth provider (google, github, ...), and if login succeeds, the user will
// be directed back to the URL you configured with WithTarget.
//
// This URL needs to invoke the AuthHandler, so it can verify that the redirect is
// legitimate, and set all parameters correctly.
//
// The default AuthHandler here will verify the parameters, and redict the user to
// the target you configured via LoginHandler.
//
// If no such target was provided, the user will just get an empty page.
// In case of error, an ugly error message is displayed.
//
// Use MakeAuthHandler to customize the behavior.
func (a *Authenticator) AuthHandler() khttp.FuncHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := a.PerformAuth(w, r)
		if err != nil {
			http.Error(w, "your lack of authentication cookie is impressive - something went wrong", http.StatusInternalServerError)
			log.Printf("ERROR - could not complete authentication - %s", err)
			return
		}

		if !CheckRedirect(w, r, data) {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		}
	}
}

// LoginState represents the information passed to the oauth provider as state.
// This state is then passed back to the AuthHandler, who must verify it.
type LoginState struct {
	Secret []byte
	Target string
	State  interface{}
}

type LoginOptions struct {
	CookieOptions kcookie.Modifiers
	Target        string
	State         interface{}
}

type LoginModifier func(*LoginOptions)

func WithCookieOptions(mod ...kcookie.Modifier) LoginModifier {
	return func(lo *LoginOptions) {
		lo.CookieOptions = append(lo.CookieOptions, mod...)
	}
}
func WithTarget(target string) LoginModifier {
	return func(lo *LoginOptions) {
		lo.Target = target
	}
}
func WithState(state interface{}) LoginModifier {
	return func(lo *LoginOptions) {
		lo.State = state
	}
}

type LoginModifiers []LoginModifier

func (lm LoginModifiers) Apply(lo *LoginOptions) *LoginOptions {
	for _, m := range lm {
		m(lo)
	}
	return lo
}

// PerformLogin writes the response to the request to actually perform the login.
func (a *Authenticator) PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error {
	options := LoginModifiers(lm).Apply(&LoginOptions{})
	url, secret, err := a.LoginURL(options.Target, options.State)
	if err != nil {
		return err
	}

	authcookie, err := a.authEncoder.Encode(secret)
	if err != nil {
		return err
	}

	http.SetCookie(w, options.CookieOptions.Apply(&http.Cookie{
		Name:     authEncoder(a.baseCookie),
		Value:    string(authcookie),
		HttpOnly: true,
	}))

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

type AuthData struct {
	Creds      *CredentialsCookie
	Identities []Identity
	Cookie     string
	Target     string
	State      interface{}
}

func (a *Authenticator) ExtractAuth(w http.ResponseWriter, r *http.Request) (AuthData, error) {
	cookie, err := r.Cookie(authEncoder(a.baseCookie))
	if err != nil || cookie == nil {
		return AuthData{}, fmt.Errorf("Cookie parsing failed - %w", err)
	}

	var secretExpected []byte
	if _, err := a.authEncoder.Decode(context.Background(), []byte(cookie.Value), &secretExpected); err != nil {
		return AuthData{}, fmt.Errorf("Cookie decoding failed - %w", err)
	}

	query := r.URL.Query()
	state := query.Get("state")
	var received LoginState
	if _, err := a.authEncoder.Decode(context.Background(), []byte(state), &received); err != nil {
		return AuthData{}, fmt.Errorf("State decoding failed - %w", err)
	}

	// Given that the state is signed and encrypted, it should not be necessary
	// to verify it against the cookie.
	//
	// However, this ensures that the redirect received is exactly for the login
	// request performed. If multiple logins are performed, the cookie will be
	// set to the latest value, and reject any other authentication callback use.
	if !bytes.Equal(secretExpected, received.Secret) {
		return AuthData{}, fmt.Errorf("Secret did not match")
	}

	http.SetCookie(w, &http.Cookie{
		Name:   authEncoder(a.baseCookie),
		MaxAge: -1,
	})
	code := query.Get("code")
	// FIXME: needs retry logic, timeout?
	tok, err := a.conf.Exchange(oauth2.NoContext, code)
	if err != nil {
		return AuthData{}, fmt.Errorf("Could not retrieve token - %w", err)
	}
	if !tok.Valid() {
		return AuthData{}, fmt.Errorf("Invalid token retrieved")
	}

	identity := &Identity{}
	for _, verifier := range a.verifiers {
		identity, err = verifier.Verify(a.log, identity, tok)
		if err != nil {
			return AuthData{}, fmt.Errorf("Invalid token - %w", err)
		}
	}
	// For defense in depth. This should never happen if configured properly.
	if identity.Id == "" || identity.Username == "" {
		return AuthData{}, fmt.Errorf("Authentication process succeeded with no credentials")
	}

	creds := CredentialsCookie{Identity: *identity, Token: *tok}
	return AuthData{Creds: &creds, Target: received.Target, State: received.State}, nil
}

func (a *Authenticator) SetAuthCookie(ad AuthData, w http.ResponseWriter, co ...kcookie.Modifier) (AuthData, error) {
	ccookie, err := a.EncodeCredentials(*ad.Creds)
	if err != nil {
		return AuthData{}, err
	}
	http.SetCookie(w, a.CredentialsCookie(ccookie, co...))
	return AuthData{Creds: ad.Creds, Cookie: ccookie, Target: ad.Target, State: ad.State}, nil
}

// Complete verifies that AuthData is well formed and valid.
//
// It checks that the identities match, and that the type of credentials are
// the same type that should be returned by the authenticator.
func (a *Authenticator) Complete(data AuthData) bool {
	if _, _, err := a.ParseCredentialsCookie(data.Cookie); err != nil {
		return false
	}
	if data.Creds == nil {
		return false
	}
	if !data.Creds.Token.Valid() {
		return false
	}
	return true
}

// CredentialsCookie will create an http.Cookie object containing the user credentials.
func (a *Authenticator) CredentialsCookie(value string, co ...kcookie.Modifier) *http.Cookie {
	return cookie.CredentialsCookie(a.baseCookie, value, co...)
}

// PerformAuth implements the logic to handle an oauth request from an oauth provider.
//
// It extracts the "state" query parameter and validates it against the state cookie,
// invoking (if configured) a validator instantiated WithFactory().
//
// In case of error, error is returned, and the rest of the fields are undefined.
//
// In a single authenticator, it always returns true because in a single authenticator it is always the last one.
func (a *Authenticator) PerformAuth(w http.ResponseWriter, r *http.Request, co ...kcookie.Modifier) (AuthData, error) {
	auth, err := a.ExtractAuth(w, r)
	if err != nil {
		return AuthData{}, err
	}

	auth, err = a.SetAuthCookie(auth, w, co...)
	if err != nil {
		return AuthData{}, err
	}

	return auth, nil
}

// authEncoder returns the name of the authentication cookie.
//
// The authentication cookie is only used to verify the correctness of the redirect,
// nothing else. It will be removed as soon as the authentication is complete.
func authEncoder(namespace string) string {
	return namespace + "Auth"
}

// CredentialsCookieName returns the name of the cookie maintaing the set of user credentials.
//
// This cookie is the one used to determine what the user can and cannot do on the UI.
func (a *Extractor) CredentialsCookieName() string {
	return cookie.CredentialsCookieName(a.baseCookie)
}

// CheckRedirect checks AuthData to see if its state warrants a redirect.
// Returns true if it did redirect, false if a redirect was unnecessary.
func CheckRedirect(w http.ResponseWriter, r *http.Request, ad AuthData) bool {
	if ad.Target == "" {
		return false
	}
	http.Redirect(w, r, ad.Target, http.StatusTemporaryRedirect)
	return true
}
