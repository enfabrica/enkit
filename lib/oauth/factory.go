package oauth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/token"
	"golang.org/x/oauth2"
)

type ExtractorFlags struct {
	// Version of the cookie format.
	Version int

	BaseCookie string

	SymmetricKey      []byte
	TokenVerifyingKey []byte

	// When generating credentials, how long should the token be valid for?
	LoginTime time.Duration
	// When checking credentials, tokens older than MaxLoginTime will be
	// rejected no matter what.
	MaxLoginTime time.Duration
}

func (f *ExtractorFlags) Register(set kflags.FlagSet, prefix string) *ExtractorFlags {
	set.IntVar(&f.Version, prefix+"token-version", f.Version,
		"Which kind of token to generate. 0 indicates version 0, 1 indicates version 1")
	set.DurationVar(&f.LoginTime, prefix+"login-time", f.LoginTime,
		"How long should the generated authentication tokens be valid for.")
	set.DurationVar(&f.MaxLoginTime, prefix+"max-login-time", f.MaxLoginTime,
		"When verifying a cookie, reject cookies older than this long no matter what.")
	set.StringVar(&f.BaseCookie, prefix+"base-cookie", "",
		"Prefix to append to the cookies used for authentication")
	set.ByteFileVar(&f.SymmetricKey, prefix+"token-encryption-key", "",
		"Path of the file containing the symmetric key to use to encrypt/decrypt returned client tokens. "+
			"If not supplied, a new key is generated")
	set.ByteFileVar(&f.TokenVerifyingKey, prefix+"token-verifying-key", "",
		"Path of the file containing the public key to use to verify the signature of client tokens. "+
			"If both token-encryption-key and token-signing-key are not specified, a key is generated")
	return f
}

func DefaultExtractorFlags() *ExtractorFlags {
	o := DefaultOptions(nil)
	return &ExtractorFlags{
		LoginTime:    o.loginTime,
		MaxLoginTime: o.maxLoginTime,
	}
}

type SigningExtractorFlags struct {
	*ExtractorFlags

	// Keys used to generate signed tokens.
	TokenSigningKey []byte
}

func (f *SigningExtractorFlags) Register(set kflags.FlagSet, prefix string) *SigningExtractorFlags {
	set.ByteFileVar(&f.TokenSigningKey, prefix+"token-signing-key", "",
		"Path of the file containing the private key to use to sign the returned client tokens. "+
			"If both token-encryption-key and token-signing-key are not specified, a key is generated")

	f.ExtractorFlags.Register(set, prefix)
	return f
}

func DefaultSigningExtractorFlags() *SigningExtractorFlags {
	return &SigningExtractorFlags{
		ExtractorFlags: DefaultExtractorFlags(),
	}
}

type RedirectorFlags struct {
	*ExtractorFlags
	AuthURL string
}

func DefaultRedirectorFlags() *RedirectorFlags {
	return &RedirectorFlags{
		ExtractorFlags: DefaultExtractorFlags(),
	}
}

func (rf *RedirectorFlags) Register(set kflags.FlagSet, prefix string) *RedirectorFlags {
	rf.ExtractorFlags.Register(set, prefix)
	set.StringVar(&rf.AuthURL, "auth-url", "", "Where to redirect users for authentication.")
	return rf
}

type Flags struct {
	*SigningExtractorFlags

	// The URL at the end of the oauth authentication process.
	TargetURL string

	// A buffer containing a JSON file with the Credentials struct (below).
	// This is passed to WithFileSecrets().
	OauthSecretJSON []byte

	// Alternative to OauthSecretJSON, OauthSecretID and OauthSecretKey can be used.
	OauthSecretID  string
	OauthSecretKey string

	// How long is the token used to authenticate with the oauth servers.
	// Limit the total time a login can take.
	AuthTime time.Duration
}

func DefaultFlags() *Flags {
	o := DefaultOptions(nil)
	return &Flags{
		SigningExtractorFlags: DefaultSigningExtractorFlags(),
		AuthTime:              o.authTime,
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.ByteFileVar(&f.OauthSecretJSON, prefix+"secret-file", "",
		"Path of the file containing the oauth credentials to use with the remote auth provider")
	set.StringVar(&f.OauthSecretID, prefix+"secret-id", "",
		"Prefer using the --"+prefix+"secret-file option - as it hides the secret from 'ps'. ID of the client to use with the oauth provider")
	set.StringVar(&f.OauthSecretKey, prefix+"secret-key", "",
		"Prefer using the --"+prefix+"secret-file option - as it hides the secret from 'ps'. Secret key of the client to use with the oauth provider")
	set.DurationVar(&f.AuthTime, prefix+"auth-time", f.AuthTime,
		"How long should the token forwarded to the remote oauth server be valid for. This bounds how long the oauth authentication process can take at most")

	f.SigningExtractorFlags.Register(set, prefix)
	return f
}

// Credentials structs are generally read from json files.
// They contain the oauth credentials used by the remote service to recognize the client.
type Credentials struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type Modifier func(auth *Options) error
type Modifiers []Modifier

func (mods Modifiers) Apply(o *Options) error {
	for _, m := range mods {
		if err := m(o); err != nil {
			return err
		}
	}
	return nil
}

func WithAuthURL(url *url.URL) Modifier {
	return func(opt *Options) error {
		opt.authURL = url
		return nil
	}
}

func WithTargetURL(url string) Modifier {
	return func(opt *Options) error {
		opt.conf.RedirectURL = url
		return nil
	}
}

func WithScopes(scopes []string) Modifier {
	return func(opt *Options) error {
		opt.conf.Scopes = append([]string{}, scopes...)
		return nil
	}
}

func WithSecrets(cid, csecret string) Modifier {
	return func(opt *Options) error {
		if cid != "" {
			opt.conf.ClientID = cid
		}

		if csecret != "" {
			opt.conf.ClientSecret = csecret
		}
		return nil
	}
}

func WithSecretJSON(data []byte) Modifier {
	return func(opt *Options) error {
		var cred Credentials
		if err := json.Unmarshal(data, &cred); err != nil {
			return err
		}
		return WithSecrets(cred.ID, cred.Secret)(opt)
	}
}

func WithSecretFile(path string) Modifier {
	return func(opt *Options) error {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		var cred Credentials
		if err := json.Unmarshal(data, &cred); err != nil {
			return err
		}
		return WithSecrets(cred.ID, cred.Secret)(opt)
	}
}

func WithEndpoint(endpoint oauth2.Endpoint) Modifier {
	return func(opt *Options) error {
		opt.conf.Endpoint = endpoint
		return nil
	}
}

// WithFactory configures a validation factory.
//
// Mandatory. Must be invoked after secrets have been configured.
func WithFactory(factory VerifierFactory) Modifier {
	return func(opt *Options) error {
		verifier, err := factory(opt.conf)
		if err != nil {
			return err
		}

		opt.verifier = verifier
		return nil
	}
}

func WithModifiers(mods ...Modifier) Modifier {
	return func(opt *Options) error {
		for _, m := range mods {
			if err := m(opt); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithSymmetricOptions(mods ...token.SymmetricSetter) Modifier {
	return func(opt *Options) error {
		opt.symmetricSetters = append(opt.symmetricSetters, mods...)
		return nil
	}
}

func WithSigningOptions(mods ...token.SigningSetter) Modifier {
	return func(opt *Options) error {
		opt.signingSetters = append(opt.signingSetters, mods...)
		return nil
	}
}

func WithAuthTime(at time.Duration) Modifier {
	return func(opt *Options) error {
		opt.authTime = at
		return nil
	}
}

func WithLoginTime(lt time.Duration) Modifier {
	return func(opt *Options) error {
		opt.loginTime = lt
		return nil
	}
}

func WithVersion(version int) Modifier {
	return func(opt *Options) error {
		if version != 0 && version != 1 {
			return fmt.Errorf("invalid version number %d - only 0 or 1 are valid", version)
		}
		opt.version = version
		return nil
	}
}

func WithMaxLoginTime(lt time.Duration) Modifier {
	return func(opt *Options) error {
		opt.maxLoginTime = lt
		return nil
	}
}

func WithRedirectorFlags(fl *RedirectorFlags) Modifier {
	return func(o *Options) error {
		authURL := fl.AuthURL
		if authURL == "" {
			return kflags.NewUsageErrorf("--auth-url must be supplied")
		}

		if strings.Index(authURL, "//") < 0 {
			authURL = "https://" + authURL
		}

		u, err := url.Parse(authURL)
		if err != nil || u.Host == "" {
			return kflags.NewUsageErrorf("invalid url %s supplied with --auth-url: %w", fl.AuthURL, err)
		}

		WithAuthURL(u)(o)
		return WithExtractorFlags(fl.ExtractorFlags)(o)
	}
}

func WithRng(rng *rand.Rand) Modifier {
	return func(o *Options) error {
		o.rng = rng
		return nil
	}
}

func WithExtractorFlags(fl *ExtractorFlags) Modifier {
	return func(o *Options) error {
		mods := []Modifier{WithCookiePrefix(fl.BaseCookie)}
		if len(fl.TokenVerifyingKey) != 0 {
			key, err := token.VerifyingKeyFromSlice(fl.TokenVerifyingKey)
			if err != nil {
				return fmt.Errorf("invalid key specified with --token-verifying-key - %s", err)
			}
			mods = append(mods, WithSigningOptions(token.UseVerifyingKey(key)))
		}

		mods = append(mods, WithSymmetricOptions(token.UseSymmetricKey(fl.SymmetricKey)), WithLoginTime(fl.LoginTime), WithMaxLoginTime(fl.MaxLoginTime), WithVersion(fl.Version))
		return Modifiers(mods).Apply(o)
	}
}

func WithSigningExtractorFlags(fl *SigningExtractorFlags) Modifier {
	return func(o *Options) error {
		mods := []Modifier{}
		if len(fl.TokenSigningKey) != 0 {
			key, err := token.SigningKeyFromSlice(fl.TokenSigningKey)
			if err != nil {
				return fmt.Errorf("invalid key specified with --token-signing-key - %s", err)
			}
			mods = append(mods, WithSigningOptions(token.UseSigningKey(key)))
		}
		mods = append(mods, WithExtractorFlags(fl.ExtractorFlags))
		return Modifiers(mods).Apply(o)
	}
}

func WithCookiePrefix(prefix string) Modifier {
	return func(o *Options) error {
		o.baseCookie = prefix
		return nil
	}
}

func WithFlags(fl *Flags) Modifier {
	return func(o *Options) error {
		if len(fl.OauthSecretJSON) == 0 && (fl.OauthSecretID == "" || fl.OauthSecretKey == "") {
			return fmt.Errorf("you must specify the secret-file or (secret-key and secret-id) options")
		}
		if len(fl.TargetURL) == 0 {
			return fmt.Errorf("you must specify the target-url flag")
		}

		mods := []Modifier{WithTargetURL(fl.TargetURL)}
		if len(fl.OauthSecretJSON) > 0 {
			mods = append(mods, WithSecretJSON(fl.OauthSecretJSON))
		}

		if len(fl.SymmetricKey) == 0 {
			// 0 is the key length, causes the default length to be used.
			key, err := token.GenerateSymmetricKey(o.rng, 0)
			if err != nil {
				return fmt.Errorf("no key specified with --token-encryption-key, and generating one failed with - %w", err)
			}
			fl.SymmetricKey = key
		}

		if len(fl.TokenSigningKey) == 0 && len(fl.TokenVerifyingKey) == 0 {
			verify, sign, err := token.GenerateSigningKey(o.rng)
			if err != nil {
				return fmt.Errorf("no key specified with --token-signing-key and --token-verifying-key, and generating one failed with - %s", err)
			}
			fl.TokenSigningKey = (*sign.ToBytes())[:]
			fl.TokenVerifyingKey = (*verify.ToBytes())[:]
		}

		mods = append(mods, WithAuthTime(fl.AuthTime), WithSecrets(fl.OauthSecretID, fl.OauthSecretKey), WithSigningExtractorFlags(fl.SigningExtractorFlags))
		return Modifiers(mods).Apply(o)
	}
}

type Options struct {
	rng          *rand.Rand
	authTime     time.Duration // How long the user has to complete authentication.
	loginTime    time.Duration // How long the token is valid for after successful authentication.
	maxLoginTime time.Duration // Tokens issued more than maxLoginTime ago will always be rejected.

	version    int
	conf       *oauth2.Config
	verifier   Verifier
	baseCookie string
	authURL    *url.URL // Only used by the Redirector.

	symmetricSetters []token.SymmetricSetter
	signingSetters   []token.SigningSetter
}

func DefaultOptions(rng *rand.Rand) Options {
	return Options{
		rng:          rng,
		authTime:     time.Minute * 30,
		loginTime:    time.Hour * 24,
		maxLoginTime: time.Hour * 24 * 365,
		conf:         &oauth2.Config{},
	}
}

func (opt *Options) NewAuthenticator() (*Authenticator, error) {
	extractor, err := opt.NewExtractor()
	if err != nil {
		return nil, err
	}

	be, err := token.NewSymmetricEncoder(opt.rng, opt.symmetricSetters...)
	if err != nil {
		return nil, fmt.Errorf("error setting up authenticating cipher: %w", err)
	}

	authenticator := &Authenticator{
		Extractor: *extractor,

		rng: opt.rng,

		authEncoder: token.NewTypeEncoder(token.NewChainedEncoder(token.NewTimeEncoder(nil, opt.authTime), be, token.NewBase64UrlEncoder())),

		conf:     opt.conf,
		verifier: opt.verifier,
	}

	if authenticator.conf.RedirectURL == "" {
		return nil, fmt.Errorf("API used incorrectly - must supply a target auth url with WithTargetURL")
	}
	if authenticator.conf.ClientID == "" || authenticator.conf.ClientSecret == "" {
		return nil, fmt.Errorf("API used incorrectly - must supply secrets with WithSecrets")
	}
	if authenticator.verifier == nil {
		return nil, fmt.Errorf("API used incorrectly - must supply verifier with WithFactory")
	}
	if len(authenticator.conf.Scopes) == 0 {
		return nil, fmt.Errorf("API used incorrectly - no scopes configured")
	}
	if authenticator.conf.Endpoint.AuthURL == "" || authenticator.conf.Endpoint.TokenURL == "" {
		return nil, fmt.Errorf("API used incorrectly - endpoint has no AuthURL or TokenURL - %#v", authenticator.conf.Endpoint)
	}

	return authenticator, nil
}

// NewExtractor creates either a simple Extractor, or a SigningExtractor.
//
// An Extractor is an object able to parse and extract data from a signed and
// encrypted cookie.
//
// A SigningExtractor is just like an extractor, except it is also capable
// of generating new signing cookies.
func (opt *Options) NewExtractor() (*Extractor, error) {
	be, err := token.NewSymmetricEncoder(opt.rng, opt.symmetricSetters...)
	if err != nil {
		return nil, fmt.Errorf("error setting up symmetric cipher: %w", err)
	}

	se, err := token.NewSigningEncoder(opt.rng, opt.signingSetters...)
	if err != nil {
		return nil, fmt.Errorf("error setting up signing cipher: %w", err)
	}

	ue := token.NewBase64UrlEncoder()
	return &Extractor{
		version:       opt.version,
		baseCookie:    opt.baseCookie,
		loginEncoder0: token.NewTypeEncoder(token.NewChainedEncoder(token.NewTimeEncoder(nil, opt.loginTime), be, se, ue)),
		loginEncoder1: token.NewTypeEncoder(token.NewChainedEncoder(token.NewTimeEncoder(nil, opt.maxLoginTime), token.NewExpireEncoder(nil, opt.loginTime), be, se, ue)),
	}, nil
}

func (opt *Options) NewRedirector() (*Redirector, error) {
	extractor, err := opt.NewExtractor()
	if err != nil {
		return nil, err
	}
	if opt.authURL == nil {
		return nil, fmt.Errorf("API usage error - an authURL must be supplied with WithAuthURL")
	}

	return &Redirector{
		Extractor: extractor,
		AuthURL:   opt.authURL,
	}, nil
}

func NewRedirector(modifiers ...Modifier) (*Redirector, error) {
	options := DefaultOptions(nil)
	if err := Modifiers(modifiers).Apply(&options); err != nil {
		return nil, err
	}

	return options.NewRedirector()
}

func NewExtractor(modifiers ...Modifier) (*Extractor, error) {
	options := DefaultOptions(nil)
	if err := Modifiers(modifiers).Apply(&options); err != nil {
		return nil, err
	}

	return options.NewExtractor()
}

func New(rng *rand.Rand, modifiers ...Modifier) (*Authenticator, error) {
	options := DefaultOptions(rng)
	if err := Modifiers(modifiers).Apply(&options); err != nil {
		return nil, err
	}
	return options.NewAuthenticator()
}
