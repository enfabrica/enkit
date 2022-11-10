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
        Google *ogoogle.Flags

	// The name of the provider to use: google or github.
	Provider string

	// Only groups matching this regex are kept.
	GroupsKeep    string
	// Name of the group will be mangled based on this substitution.
	GroupsRename  string
}

func DefaultFlags() *Flags {
	return &Flags{
		Flags: oauth.DefaultFlags(),
		Google: ogoogle.DefaultFlags(),
		Provider: "google",

		GroupsKeep: "role-([^@]*)@.*",
		GroupsRename: "$1",
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	f.Flags.Register(set, prefix)
	f.Google.Register(set, prefix+"google-")

	set.StringVar(&f.Provider, prefix+"provider", f.Provider,
		"Selects the provider to use, one of 'google' or 'github'")

	set.StringVar(&f.GroupsKeep, prefix+"groups-keep", f.GroupsKeep,
		"If set, only groups matching this regular expression will be propagated into the user identity")
	set.StringVar(&f.GroupsRename, prefix+"groups-rename", f.GroupsRename,
		"If set, each group name will be replaced with this expression - a regex replace expression based on groups-keep")

	return f
}

func WithFlags(fl *Flags) oauth.Modifier {
	return func(o *oauth.Options) error {
		if err := oauth.WithFlags(fl.Flags)(o); err != nil {
			return err
		}

		var err error
		switch fl.Provider {
		case "google":
			mod, err := ogoogle.FromFlags(fl.Google)
			if err != nil {
				return fmt.Errorf("could not initialize google provider (--provider=google): %w", err)
			}
			err = mod(o)

		case "github":
			err = ogithub.Defaults()(o)
		default:
			return fmt.Errorf("unknown provider: %s specified with --provider. Valid: google, github")
		}

		if err != nil {
			return fmt.Errorf("provider %s returned error: %w", fl.Provider, err)
		}

		if fl.GroupsKeep != "" {
			gf, err := NewGroupsKeeperFactory(fl.GroupsKeep, fl.GroupsRename)
			if err != nil {
				return fmt.Errorf("invalid --groups-keep or --groups-rename flag: %w", err)
			}

			if err := oauth.WithFactory(gf)(o); err != nil {
				return err
			}
		}

		return nil
	}
}
