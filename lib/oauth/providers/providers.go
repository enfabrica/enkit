// Package providers provides functions to configure and use the providers supported
// out of the box by the enkit oauth library: google and github.
//
// Use the functions in this file to easily bring up a working authentication
// server or client almost entirely controlled by flags.
package providers

import (
	"fmt"

	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogoogle"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"github.com/enfabrica/enkit/lib/kflags"
)

// Flags allows to configure oauth for one of the specific providers
// supported by the library out of the box.
//
// To pass Flags to one of the constructurs, use `WithFlags`.
type Flags struct {
	*oauth.Flags

	// The name of the provider to use: google or github.
	Provider string
}

func DefaultFlags() *Flags {
	return &Flags{
		Flags: oauth.DefaultFlags(),
		Provider: "google",
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	f.Flags.Register(set, prefix)

	set.StringVar(&f.Provider, prefix+"provider", f.Provider,
		"Selects the provider to use, one of 'google' or 'github'")
	return f
}

func WithFlags(fl *Flags) oauth.Modifier {
	return func(o *oauth.Options) error {
		if err := oauth.WithFlags(fl.Flags)(o); err != nil {
			return err
		}

		switch fl.Provider {
		case "google":
			return ogoogle.Defaults()(o)
		case "github":
			return ogithub.Defaults()(o)
		}
		return fmt.Errorf("unknown provider: %s specified with --provider. Valid: google, github")
	}
}
