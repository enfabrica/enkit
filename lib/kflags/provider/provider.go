package provider

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kconfig"
	"github.com/enfabrica/enkit/lib/khttp/downloader"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"github.com/enfabrica/enkit/lib/logger"

	"github.com/enfabrica/enkit/lib/cache"
	"net/http"
)

type ProviderFlags kconfig.Flags

func DefaultProviderFlags() *ProviderFlags {
	return (*ProviderFlags)(kconfig.DefaultFlags())
}

func (pf *ProviderFlags) Register(set kflags.FlagSet, prefix string) *ProviderFlags {
	((*kconfig.Flags)(pf)).Register(set, prefix)
	return pf
}

type Options struct {
	Log    logger.Logger
	Cookie *http.Cookie

	Cache cache.Store

	CommandName string
	Domain      string
}

func SetFlagDefaults(populator kflags.Populator, flags *ProviderFlags, options *Options) error {
	mods := []kconfig.Modifier{kconfig.WithLogger(options.Log)}
	if options.Cookie != nil {
		mods = append(mods, kconfig.WithGetOptions(downloader.WithRequestOptions(krequest.WithCookie(options.Cookie))))
	}
	mods = append(mods, kconfig.FromFlags((*kconfig.Flags)(flags)))

	resolver, err := kconfig.NewConfigAugmenterFromDNS(options.Cache, options.Domain, options.CommandName, mods...)
	if err != nil {
		return err
	}

	err = populator(resolver)
	if err != nil {
		return err
	}

	return nil
}
