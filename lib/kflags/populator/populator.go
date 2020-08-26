package populator

import (
	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kconfig"
	"github.com/enfabrica/enkit/lib/khttp/downloader"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/logger/klog"
	"github.com/enfabrica/enkit/lib/oauth/cookie"

	"github.com/enfabrica/enkit/lib/cache"
	"os"
)

type Populator struct {
	populator     kflags.Populator
	klogflags     *klog.Flags
	resolverflags *kconfig.Flags
	identityflags *identity.Flags
	cs            *cache.Local

	domain     string
	authcookie string
}

func New(label string, populator kflags.Populator) *Populator {
	return &Populator{
		populator:     populator,
		klogflags:     klog.DefaultFlags(),
		identityflags: identity.DefaultFlags(),
		cs:            cache.NewLocal(label),
		resolverflags: kconfig.DefaultFlags(),
	}
}

func (p *Populator) Register(fs kflags.FlagSet, prefix string) *Populator {
	p.klogflags.Register(fs, prefix+"enkit-")
	p.identityflags.Register(fs, prefix+"enkit-")
	p.cs.Register(fs, prefix+"enkit-")
	p.resolverflags.Register(fs, prefix+"enkit-")

	fs.StringVar(&p.domain, prefix+"enkit-domain", "", "Default domain name to use to retrieve the default configurations.")
	fs.StringVar(&p.domain, prefix+"enkit-authcookie", "", "Default prefix for the authentication cookie.")

	return p
}

func (p *Populator) NewLogger(name string) (logger.Logger, error) {
	return klog.New(name, klog.FromFlags(*p.klogflags))
}

func (p *Populator) PopulateDefaults(name string) (logger.Logger, error) {
	errs := p.populator(kflags.NewEnvResolver(name))

	logger, err := p.NewLogger(name)
	if err != nil {
		return nil, err
	}
	if errs != nil {
		logger.Warnf("Parsing command line flags from environment variables resulted in %s", errs)
	}

	ids, err := identity.NewStore(defcon.Open)

	id, token, err := ids.Load(p.identityflags.Identity())
	if err != nil && !os.IsNotExist(err) {
		logger.Warnf("Loading default identity from disk failed %s", err)
	}

	_, domain := identity.SplitUsername(id, p.domain)
	if domain == "" {
		logger.Infof("Unknown domain / identity at startup - not fetching remote configuration")
		return logger, nil
	}

	return logger, p.PopulateDefaultsForOptions(name, &Options{
		Token:  token,
		Domain: domain,
		Logger: logger,
	})
}

type Options struct {
	Token  string
	Domain string
	Logger logger.Logger
}

func (p *Populator) PopulateDefaultsForOptions(name string, opts *Options) error {
	if opts.Logger == nil {
		opts.Logger = logger.Nil
	}

	// 2) See if we know the domain name we can use to fetch the defaults, and cache.
	resolver, err := kconfig.NewConfigResolverFromDNS(p.cs, opts.Domain, name,
		kconfig.WithLogger(opts.Logger),
		kconfig.WithGetOptions(downloader.WithRequestOptions(krequest.WithCookie(cookie.CredentialsCookie(p.authcookie, opts.Token)))),
		kconfig.FromFlags(p.resolverflags))
	if err != nil {
		opts.Logger.Infof("Error fetching remote configuration, no remote defaults - %s", err)
		return err
	}

	err = p.populator(resolver)
	if err != nil {
		opts.Logger.Warnf("Error populating remote defaults - %s", err)
	}

	return err
}
