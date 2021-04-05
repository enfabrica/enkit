package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/client/auth"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"math/rand"
	"time"
)

type Login struct {
	*cobra.Command
	rng *rand.Rand

	base      *client.BaseFlags
	populator kflags.Populator

	NoDefault   bool
	MinWaitTime time.Duration
}

// NewLogin creates a new Login command.
//
// Base is the pointer to a base object, initialized with NewBase.
// rng is a secure random number generator.
//
// When the login command is run, it will:
// - apply the configuration defaults necessary for the domain, using a populator.
// - retrieve an authentication token from the authentication server.
// - save it on disk, optionally as a default identity.
func NewLogin(base *client.BaseFlags, rng *rand.Rand, populator kflags.Populator) *Login {
	login := &Login{
		Command: &cobra.Command{
			Use:     "login",
			Short:   "Retrieve credentials to access the artifact repository",
			Aliases: []string{"auth", "hello", "hi"},
		},
		base:      base,
		rng:       rng,
		populator: populator,
	}
	login.Command.RunE = login.Run

	login.Flags().BoolVarP(&login.NoDefault, "no-default", "n", false, "Do not mark this identity as the default identity to use")
	login.Flags().DurationVar(&login.MinWaitTime, "min-wait-time", 10*time.Second, "Wait at least this long in between failed attempts to retrieve a token")
	return login
}

func (l *Login) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return kflags.NewUsageErrorf("use as 'astore login username@domain.com' or just '@domain.com' - exactly one argument")
	}

	ids, err := l.base.IdentityStore()
	if err != nil {
		return fmt.Errorf("could not open identity store - %w", err)
	}

	argname := l.base.Identity()
	if len(args) >= 1 {
		argname = args[0]
	} else if argname == "" {
		argname, _, _ = ids.Load("")
	}

	username, domain := identity.SplitUsername(argname, l.base.DefaultDomain)
	if domain == "" {
		return kflags.NewUsageErrorf("Please specify your 'username@domain.com' as first argument, '... login myname@mydomain.com'")
	}

	// Once we know the domain of the user, we can load the options associated with that domain.
	// Note that here we have no token yet, as the authentication process has not been started yet.
	if l.populator != nil {
		if err := l.base.UpdateFlagDefaults(l.populator, domain); err != nil {
			l.base.Log.Infof("updating default flags failed: %s", err)
		}
	}

	client, err := l.base.AuthClient(l.rng)
	if err != nil {
		return err
	}

	options := auth.LoginOptions{
		Context: l.base.Context(),
		MinWait: l.MinWaitTime,
		Store:   l.base.Local,
	}

	token, err := client.Login(username, domain, options)
	if err != nil {
		return err
	}
	userid := identity.Join(username, domain)
	err = ids.Save(userid, token)
	if err != nil {
		return fmt.Errorf("could not store identity - %w", err)
	}
	if l.NoDefault == false {
		err = ids.SetDefault(userid)
		if err != nil {
			return fmt.Errorf("could not mark identity as default - %w", err)
		}
	}

	return nil
}
