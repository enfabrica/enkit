package inviter

import (
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/ghinviter/ui"
	"github.com/enfabrica/enkit/ghinviter/assets"
	"github.com/enfabrica/enkit/lib/khttp/kassets"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"math/rand"
	"net/http"
	"log"
	"fmt"
)

type Flags struct {
	Http	*khttp.Flags

	Access *oauth.RedirectorFlags
	Github *oauth.Flags

	// The URL external clients can use to reach this website.
	//
	// It is used to compute the termination endpoint for oauth. If https
	// is enabled, it is also used to compute the domain name in the
	// certificate.
	TargetURL string
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	fl.Http.Register(set, prefix)
	fl.Access.Register(set, prefix+"access-")
	fl.Github.Register(set, prefix+"github-")

	set.StringVar(&fl.TargetURL, prefix+"site-url", "",
			"The URL external users can use to reach this web server - used as the oauth target "+
			"redirection endpoint, and as the domain for https certificates (if https is enabled)")

	return fl
}

func DefaultFlags() *Flags {
	flags := &Flags{
		Http: khttp.DefaultFlags(),
		Access: oauth.DefaultRedirectorFlags(),
		Github: oauth.DefaultFlags(),
	}

	return flags
}

type Inviter struct {
	log    logger.Logger

	server *khttp.Server
	redirector *oauth.Redirector
	github *oauth.Authenticator
	mux *http.ServeMux
}

type Modifier func(*Inviter) error

type Modifiers []Modifier

func (mods Modifiers) Apply(inviter *Inviter) error {
	for _, mod := range mods {
		if err := mod(inviter); err != nil {
			return err
		}
	}
	return nil
}

func WithServer(server *khttp.Server) Modifier {
	return func(inviter *Inviter) error {
		inviter.server = server
		return nil
	}
}

func WithLogger(log logger.Logger) Modifier {
	return func(inviter *Inviter) error {
		inviter.log = log
		return nil
	}
}

func WithRedirector(redir *oauth.Redirector) Modifier {
	return func(inviter *Inviter) error {
		inviter.redirector = redir
		return nil
	}
}

func WithGithub(auth *oauth.Authenticator) Modifier {
	return func(inviter *Inviter) error {
		inviter.github = auth
		return nil
	}
}

func WithMux(mux *http.ServeMux) Modifier {
	return func(inviter *Inviter) error {
		inviter.mux = mux
		return nil
	}
}

func FromFlags(rng *rand.Rand, flags *Flags) Modifier {
	return func(inviter *Inviter) error {
		mods := Modifiers{}

		if flags.TargetURL == "" {
			return fmt.Errorf("A target URL must be set with --site-url - must match one of the URLs configured in the oauth provider")
		}

		server, err := khttp.FromFlags(flags.Http)
		if err != nil {
			return err
		}
		mods = append(mods, WithServer(server))

		log.Printf("SETTING REDIRECTOR FLAGS")
		redir, err := oauth.NewRedirector(oauth.WithRedirectorFlags(flags.Access), oauth.WithTargetURL(flags.TargetURL))
		if err != nil {
			return err
		}
		mods = append(mods, WithRedirector(redir))

		log.Printf("SETTING GITHUB FLAGS")
		github, err := oauth.New(rng, oauth.WithFlags(flags.Github), oauth.WithTargetURL(flags.TargetURL), ogithub.Defaults())
		if err != nil {
			return err
		}
		mods = append(mods, WithGithub(github))

		return mods.Apply(inviter)
	}
}

func New(mods... Modifier) (*Inviter, error) {
	inviter := &Inviter{
		log: logger.Nil, // FIXME: use .Go
		mux: http.NewServeMux(),
		server: khttp.DefaultServer(),
	}

	if err := Modifiers(mods).Apply(inviter); err != nil {
		return nil, err
	}

	stats := kassets.AssetStats{}

	kassets.RegisterAssets(&stats, ui.Data, "", kassets.BasicMapper(kassets.MuxMapper(inviter.mux)))
	kassets.RegisterAssets(&stats, assets.Data, "", kassets.BasicMapper(kassets.MuxMapper(inviter.mux)))
	stats.Log(inviter.log.Infof)

	return inviter, nil
}

func (iv *Inviter) Run() error {
	return iv.server.Run(iv.log.Infof, iv.mux)
}
