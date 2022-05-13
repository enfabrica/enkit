package client

import (
	"errors"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/client/ccontext"
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/provider"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/logger/klog"
	"github.com/enfabrica/enkit/lib/oauth/cookie"
	"github.com/enfabrica/enkit/lib/progress"
	"log"
	"net/http"
)

type AuthFlags struct {
	// The identity to use.
	*identity.IdentityFlags

	// Flags indicating how to connect to the authentication server.
	*ServerFlags
}

func DefaultAuthFlags() *AuthFlags {
	return &AuthFlags{
		IdentityFlags: identity.DefaultIdentityFlags(),
		ServerFlags:   DefaultServerFlags("auth", "Authentication server", ""),
	}
}

func (af *AuthFlags) Register(flags kflags.FlagSet, prefix string) *AuthFlags {
	af.IdentityFlags.Register(flags, prefix)
	af.ServerFlags.Register(flags, prefix)
	return af
}

type BaseFlags struct {
	*klog.Flags
	*AuthFlags
	*cache.Local
	*provider.ProviderFlags

	// Function capable of opening the config stores.
	// This is not controlled by command line, but useful for tests or
	// other libraries consuming BaseFlags.
	ConfigOpener config.Opener

	// The name used to build the paths to find config files or cached files.
	// For example, if ConfigName is "enkit", config files may be stored in "~/.config/enkit".
	// This name can be shared across multiple CLI tools needing the same configs.
	ConfigName string

	// The prefix to prepend to authentication cookies.
	// This is only useful if you have multiple organizations using different instance of enkit tools.
	CookiePrefix string

	// The name of the command, used in help strings and to load config customizations specific to the command.
	CommandName string

	// Avoid displaying progress bars.
	NoProgress bool

	// Allow to override the security token used, and identity.
	OverrideToken string
	OverrideIdentity string

	// Logger object. Guaranteed to never be nil, and always be usable.
	Log *logger.Proxy
}

func DefaultBaseFlags(commandName, configName string) *BaseFlags {
	return &BaseFlags{
		ConfigOpener: defcon.Open,
		ConfigName:   configName,
		CommandName:  commandName,

		Flags:         klog.DefaultFlags(),
		AuthFlags:     DefaultAuthFlags(),
		Local:         cache.NewLocal(configName),
		ProviderFlags: provider.DefaultProviderFlags(),

		Log: &logger.Proxy{Logger: logger.NewAccumulator()},
	}
}

// Use with kcobra.Run or similar functions to decoarete an IdentityError
// with the proper error message to guide the user through authentication.
//
// For example, use kcobra.Run(..., WithErrorHandler(
//         HandleIdentityError("run enkit login to log in")
//
// to make sure the user is told to use enkit login in case of identity
// related errors.
func (bf *BaseFlags) IdentityErrorHandler(message string) kflags.ErrorHandler {
	return func(err error) error {
		var ie *kflags.IdentityError
		if !errors.As(err, &ie) {
			return err
		}
		identity := bf.Identity()
		if identity == "" {
			identity, _, _ = bf.IdentityToken()
			if identity == "" {
				identity = "youruser@yourdomain.com"
			}
		}
		return kflags.NewStatusErrorf(100,
                        "Attempting to use your credentials failed with:\n%w\n\nThis probably means that you just need to log in again with:\n\t%s %s",
			logger.NewIndentedError(err, "    (for debug only) "), message, identity)
	}
}

func (bf *BaseFlags) IdentityStore() (identity.IdentityStore, error) {
	bf.Log.Infof("Loading credentials from store '%s'", bf.ConfigName)
	id, err := identity.NewStore(bf.ConfigName, bf.ConfigOpener)
	if err != nil {
		return nil, kflags.NewIdentityError(err)
	}
	return id, nil
}

func (bf *BaseFlags) IdentityCookie() (string, *http.Cookie, error) {
	username, token, err := bf.IdentityToken()
	if err != nil {
		return "", nil, kflags.NewIdentityError(err)
	}

	return username, cookie.CredentialsCookie(bf.CookiePrefix, token), nil
}

func (bf *BaseFlags) IdentityToken() (string, string, error) {
	if bf.OverrideToken != "" || bf.OverrideIdentity != "" {
		if bf.OverrideIdentity == "" || bf.OverrideToken == "" {
			return "", "", kflags.NewUsageErrorf("if override-identity or override-token is specified, both need to be specified")
		}
		return bf.OverrideIdentity, bf.OverrideToken, nil
	}

	store, err := bf.IdentityStore()
	if err != nil {
		return "", "", err
	}

	identity := bf.Identity()
	username, token, err := store.Load(identity)
	if err != nil {
		bf.Log.Infof("Error loading credentials for '%s' - %s", bf.Printable(), err)
		return "", "", kflags.NewIdentityError(err)
	}
	bf.Log.Infof("Using credentials of '%s' for requested '%s'", username, bf.Printable())
	return username, token, nil
}

func (bf *BaseFlags) Register(set kflags.FlagSet, prefix string) *BaseFlags {
	bf.Flags.Register(set, prefix)
	bf.AuthFlags.Register(set, prefix)
	bf.Local.Register(set, prefix)
	bf.ProviderFlags.Register(set, prefix)

	set.StringVar(&bf.OverrideToken, prefix+"override-token", "", "Use this security token instead of loading one from disk")
	set.StringVar(&bf.OverrideIdentity, prefix+"override-identity", "", "Use this identity instead of loading one from disk")

	set.StringVar(&bf.CookiePrefix, prefix+"cookie-prefix", "", "Prefix to use in naming the authentication cookie. You should not normally need to change this")
	set.BoolVar(&bf.NoProgress, prefix+"no-progress", bf.NoProgress, "Disable progress bars")
	return bf
}

func (bf *BaseFlags) LoadFlagAssets(populator kflags.Populator, assets map[string][]byte) {
	populator(kflags.NewAssetAugmenter(bf.Log, bf.CommandName, assets))
}

func (bf *BaseFlags) Run(set kflags.FlagSet, populator kflags.Populator, run kflags.Runner) {
	bf.Register(set, "")
	// At this point, all flags have the default value set from the .go files.
	// Change the defaults based on environment variables.
	if err := populator(kflags.NewEnvAugmenter()); err != nil {
		bf.Log.Infof("Setting default flags from environment failed with: %s", err)
	}

	// Now that we have (possibly) user chosen defaults, load the defaults
	// from the configured default provider.
	//
	// This will likely result in fetching the flags from https/astore.
	if err := bf.UpdateFlagDefaults(populator, ""); err != nil {
		bf.Log.Infof("Updating default flags for domain failed with: %s", err)
	}

	// Finally, run the command.
	run(set, bf.Log.Infof, bf.Init)
}

// Initializes a BaseFlags object after all flags have been parsed.
//
// Invoked automatically by Run once defaults are loaded.
// Must be invoked every time base flags change value, to refresh the corresponding objects.
func (bf *BaseFlags) Init() error {
	// The newly loaded flags may change how logging needs to be performed.
	// Let's recreate the logging objects.
	var newlog logger.Logger
	newlog, err := klog.New(bf.CommandName, klog.FromFlags(*bf.Flags))
	if err != nil {
		bf.Log.Infof("could not initialize logger - %s", err)
		newlog = &logger.DefaultLogger{Printer: log.Printf}
	}

	bf.Log.Replace(newlog)
	return err
}

// UpdateFlagDefaults updates the default value of flags by fetching the
// configuration from an https/astore server.
func (bf *BaseFlags) UpdateFlagDefaults(populator kflags.Populator, domain string) error {
	// Try to load an authentication cookie before even trying.
	// This may just work based on env variables, or previously loaded defaults, but
	// it's optional - keep going if this fails.
	username, cookie, err := bf.IdentityCookie()
	if err != nil {
		bf.Log.Infof("could not retrieve authentication cookie - continuing without (error: %s)", err)
	}
	if domain == "" {
		_, domain = identity.SplitUsername(username, bf.Domain())
	}

	options := &provider.Options{
		Log:    bf.Log,
		Cookie: cookie,

		Cache: bf.Local,

		CommandName: bf.CommandName,
		Domain:      domain,
	}

	// Load the new flags, and re-initialize the internal objects.
	fallback, err := bf.ConfigOpener(bf.ConfigName, "fallback")
	if err != nil {
		return err
	}
	if err := provider.SetFlagDefaults(fallback, populator, bf.ProviderFlags, options); err != nil {
		bf.Log.Infof("could not retrieve remote defaults - continuing without (error: %s)", err)
	}
	bf.Init()
	return nil
}

// Context() creates a new Context object.
func (bf *BaseFlags) Context() *ccontext.Context {
	context := ccontext.DefaultContext()

	context.Logger = bf.Log
	if bf.NoProgress {
		context.Progress = progress.NewDiscard
	} else {
		context.Progress = progress.NewBar
	}

	return context
}
