package commands

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/client/auth"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/kflags/populator"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"math/rand"
	"time"
)

type Base struct {
	client.CommonFlags

	Populator *populator.Populator
	Log       logger.Logger

	auth client.ServerFlags
}

func NewBase() *Base {
	return &Base{}
}

func (rc *Base) Options() *client.CommonOptions {
	return rc.CommonFlags.Options(rc.Log)
}

func (rc *Base) AuthClient(rng *rand.Rand) (*auth.Client, error) {
	authconn, err := rc.auth.Connect()
	if err != nil {
		return nil, err
	}

	return auth.New(rng, authconn), nil
}

func (rc *Base) IdentityStore() (*identity.Identity, error) {
	return identity.NewStore(defcon.Open)
}

func (rc *Base) Register(set *pflag.FlagSet) {
	rc.auth.Register(set, "auth", "Authentication server", "")
	rc.CommonFlags.Register(set)
}

type Login struct {
	*cobra.Command
	rng *rand.Rand

	root *Base
	name string

	DefaultDomain string
	NoDefault     bool
	MinWaitTime   time.Duration
}

func NewLogin(root *Base, name string, rng *rand.Rand) *Login {
	login := &Login{
		Command: &cobra.Command{
			Use:     "login",
			Short:   "Retrieve credentials to access the artifact repository",
			Aliases: []string{"auth", "hello", "hi"},
		},
		root: root,
		name: name,
		rng:  rng,
	}
	login.Command.RunE = login.Run

	login.Flags().StringVar(&login.DefaultDomain, "default-domain", "", "Default domain to use, in case the username does not specify one")
	login.Flags().BoolVarP(&login.NoDefault, "no-default", "n", false, "Do not mark this identity as the default identity to use")
	login.Flags().DurationVar(&login.MinWaitTime, "min-wait-time", 10*time.Second, "Wait at least this long in between failed attempts to retrieve a token")

	return login
}

func (l *Login) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return kcobra.NewUsageError(fmt.Errorf("use as 'astore login username@domain.com' or just '@domain.com' - exactly one argument"))
	}

	ids, err := l.root.IdentityStore()
	if err != nil {
		return fmt.Errorf("could not open identity store - %w", err)
	}

	argname := ""
	if len(args) >= 1 {
		argname = args[0]
	} else {
		argname, _, _ = ids.Load("")
	}

	username, domain := identity.SplitUsername(argname, l.DefaultDomain)
	if domain == "" {
		return kcobra.NewUsageError(fmt.Errorf("no domain found from either --default-domain or the supplied username '%s' - must specify 'username@domain.com' as argument", username))
	}

	l.root.Populator.PopulateDefaultsForOptions(l.name, &populator.Options{
		Token:  "",
		Domain: domain,
		Logger: l.root.Log,
	})

	client, err := l.root.AuthClient(l.rng)
	if err != nil {
		return err
	}

	options := auth.LoginOptions{
		CommonOptions: l.root.Options(),
		MinWait:       l.MinWaitTime,
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
