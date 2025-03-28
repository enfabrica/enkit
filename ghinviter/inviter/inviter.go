package inviter

import (
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogrpc"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/kgrpc"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/ghinviter/ui"
	"github.com/enfabrica/enkit/ghinviter/assets"
	"github.com/enfabrica/enkit/ghinviter/proto"
	"github.com/enfabrica/enkit/lib/khttp/kassets"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"google.golang.org/grpc"
        "google.golang.org/grpc/codes"
        "google.golang.org/grpc/status"
	"math/rand"
	"net/http"
	"fmt"
	"context"
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

	// We are using two instances of the oauth library.
	// Make sure cookie names don't conflict.
	flags.Github.BaseCookie = "Gh"

	return flags
}

type Inviter struct {
	log    logger.Logger

	server *khttp.Server
	redirector *oauth.Redirector
	github *oauth.Authenticator
	mux *http.ServeMux
	grpcs *grpc.Server
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

		// FIXME! Include the user server. Include the interceptor.
		server, err := kgrpc.NewServer(inviter.mux, inviter.grpcs, khttp.WithLogger(inviter.log.Infof), khttp.FromFlags(flags.Http))
		if err != nil {
			return err
		}
		mods = append(mods, WithServer(server))

		redir, err := oauth.NewRedirector(oauth.WithRedirectorFlags(flags.Access), oauth.WithTargetURL(flags.TargetURL))
		if err != nil {
			return err
		}
		mods = append(mods, WithRedirector(redir))

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
		grpcs: grpc.NewServer(),
		server: nil,
	}
	proto.RegisterAccountServer(inviter.grpcs, inviter)

	if err := Modifiers(mods).Apply(inviter); err != nil {
		return nil, err
	}

	if inviter.github == nil {
		return nil, fmt.Errorf("API usage error - a github oauth handler must be initialized - use WithGithub")
	}

	stats := kassets.AssetStats{}

	urlmap := map[string]kassets.Wrapper{
		"/index.html": func (handler khttp.FuncHandler) khttp.FuncHandler {
		return oauth.MakeAuthHandler(inviter.github, handler)
	},
		"": nil,
	}

	kassets.RegisterAssets(&stats, ui.Data, "/ui/", kassets.BasicMapper(oauth.Mapper(inviter.redirector, kassets.MuxMapper(inviter.mux))))
	kassets.RegisterAssets(&stats, assets.Data, "", kassets.BasicMapper(oauth.Mapper(inviter.redirector, kassets.MapWrapper(urlmap, kassets.MuxMapper(inviter.mux)))))

	inviter.mux.HandleFunc("/github", oauth.LoginHandler(inviter.github, oauth.WithTarget("/")))

	stats.Log(inviter.log.Infof)
	return inviter, nil
}

func (iv *Inviter) Run() error {
	iv.log.Infof("starting now...")
	return iv.server.Run()
}

func (iv *Inviter) GetMyUserGroups(ctx context.Context, req *proto.GetMyUserGroupsRequest) (*proto.UserGroups, error) {
	var extractor *oauth.Extractor
	var provider string
	if req.Provider == "google" || req.Provider == "primary" || req.Provider == "" {
		extractor = iv.redirector.Extractor
		provider = "google"
	} else if req.Provider == "github" || req.Provider == "secondary" {
		extractor = &iv.github.Extractor
		provider = "github"
        } else {
		return nil, status.Errorf(codes.InvalidArgument, "unknown provider: %s", req.Provider)
	}

	creds, err := ogrpc.GetCredentials(extractor, ctx)
	if err != nil {
		return nil, err
	}

	return &proto.UserGroups{
		User: &proto.User{
			Provider: provider,
			Id: creds.Identity.Id,
			Username: creds.Identity.Username,
			Organization: creds.Identity.Organization,
		},
		// FIXME: return groups!!!
	}, nil
}

func (iv *Inviter) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.User, error) {
	return &proto.User{}, nil
}
