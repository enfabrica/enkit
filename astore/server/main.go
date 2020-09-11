package main

import (
	"os"

	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/enfabrica/enkit/astore/common"
	rpc_astore "github.com/enfabrica/enkit/astore/rpc/astore"
	rpc_auth "github.com/enfabrica/enkit/astore/rpc/auth"
	"github.com/enfabrica/enkit/astore/server/assets"
	"github.com/enfabrica/enkit/astore/server/astore"
	"github.com/enfabrica/enkit/astore/server/auth"
	"github.com/enfabrica/enkit/astore/server/configs"
	"github.com/enfabrica/enkit/astore/server/credentials"
	"github.com/enfabrica/enkit/astore/server/templates"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/kflags/kconfig"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogoogle"
	"github.com/enfabrica/enkit/lib/oauth/ogrpc"
	"github.com/enfabrica/enkit/lib/server"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var messageSuccess = `You managed to type your username and password correctly. Or click the right buttons. Whatever.<br />Enjoy your credentials while they last, delivered to you directly to your terminal.`

var messageFail = `Unfortunatley, it does not seem like you are authorized to use this system.<br />Either that, or something went very wrong with the authentication process. Retry if you wish, but good luck.`

var messageError = `That's embarrassing. But it seems like something about your request did not go well.<br />I would certainly retry. Good luck.`

var messageNothing = `There's nothing to see here yet.<br />Go away, please.`

var messageNotFound = `Well, what you're looking for ain't here.<br />Go find a better URL (or a backup...). Or someone else who can give you this file.`

func ShowResult(w http.ResponseWriter, r *http.Request, image, title, message string, status int) {
	w.WriteHeader(status)
	templates.WritePageTemplate(w, &templates.MessagePage{
		PageTitle: title,
		Highlight: title,
		Text:      message,
		ImageCSS:  image,
	})
}

func DownloadHandler(base, upath string, resp *rpc_astore.RetrieveResponse, err error, w http.ResponseWriter, r *http.Request) {
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ShowResult(w, r, "hungry", "This file does not seem to exist", messageNotFound, http.StatusNotFound)
		} else {
			ShowResult(w, r, "broken", fmt.Sprintf("Well, something bad happened. Maybe you want to visit <a href='%s'>the list view</a>?", base+upath),
				messageError, http.StatusInternalServerError)
		}
		return
	}

	// Ensure that the file, when the browser goes to download it, has the original file name, rather than the uid.
	// Wget and curl get it right, surprisingly, as they stick to the original file name from the URL before the redirect.
	// If it wasn't for that, they'd need the --content-disposition or -J flag.
	disposition := "&response-content-disposition=" + url.PathEscape(fmt.Sprintf(`inline; filename="%s"`, path.Base(upath)))
	http.Redirect(w, r, resp.Url+disposition, http.StatusTemporaryRedirect)
}

func ListHandler(base, upath string, resp *rpc_astore.ListResponse, err error, w http.ResponseWriter, r *http.Request) {
	if err != nil {
		if status.Code(err) == codes.NotFound {
			ShowResult(w, r, "hungry", "This file does not seem to exist", messageNotFound, http.StatusNotFound)
		} else {
			ShowResult(w, r, "broken", "Something bad happened", messageError, http.StatusInternalServerError)
		}
		return
	}

	templates.WritePageTemplate(w, &templates.ListPage{
		PageTitle: upath + " download",
		Path:      upath,
		List:      resp,
		Base:      base,
	})
}

func Start(targetURL, cookieDomain string, astoreFlags *astore.Flags, authFlags *auth.Flags, oauthFlags *oauth.Flags) error {
	rng := rand.New(srand.Source)

	cookieDomain = strings.TrimSpace(cookieDomain)
	if cookieDomain != "" && !strings.HasPrefix(cookieDomain, ".") {
		cookieDomain = "." + cookieDomain
	}

	if targetURL == "" {
		return kcobra.NewUsageError(fmt.Errorf("--site-url must be specified"))
	}

	// Adjust the URLs the user supplied based on what the web server below does.
	authFlags.AuthURL = strings.TrimSuffix(targetURL, "/") + "/a/"
	oauthFlags.TargetURL = strings.TrimSuffix(targetURL, "/") + "/e/"

	listURL := ""
	downloadURL := ""
	if astoreFlags.PublishBaseURL != "" {
		trimmed := strings.TrimSuffix(astoreFlags.PublishBaseURL, "/")
		listURL = trimmed + "/l/"
		downloadURL = trimmed + "/d/"
		astoreFlags.PublishBaseURL = listURL
	}

	astoreServer, err := astore.New(rng, astore.WithFlags(astoreFlags))
	if err != nil {
		return fmt.Errorf("could not initialize storage - %s Maybe you need to pass --credentials-file or --project-id-file?", err)
	}

	authServer, err := auth.New(rng, auth.WithFlags(authFlags))
	if err != nil {
		return fmt.Errorf("could not initialize auth server - %s", err)
	}

	authWeb, err := oauth.New(rng, oauth.WithFlags(oauthFlags), ogoogle.Defaults())
	if err != nil {
		return fmt.Errorf("could not initialize oauth authenticator - %s", err)
	}

	grpcs := grpc.NewServer(
		grpc.StreamInterceptor(ogrpc.StreamInterceptor(authWeb, "/auth.Auth/")),
		grpc.UnaryInterceptor(ogrpc.UnaryInterceptor(authWeb, "/auth.Auth/")),
	)
	rpc_astore.RegisterAstoreServer(grpcs, astoreServer)
	rpc_auth.RegisterAuthServer(grpcs, authServer)

	mux := http.NewServeMux()
	stats := server.AssetStats{}
	mux.HandleFunc("/configs/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("%s On %s", kconfig.YodaSays, time.Now()), http.StatusNotFound)
	})
	mux.HandleFunc("/l/", func(w http.ResponseWriter, r *http.Request) {
		astoreServer.ListPublished("/l/", func(upath string, resp *rpc_astore.ListResponse, err error, w http.ResponseWriter, r *http.Request) {
			ListHandler(downloadURL, upath, resp, err, w, r)
		}, w, r)
	})
	mux.HandleFunc("/d/", func(w http.ResponseWriter, r *http.Request) {
		astoreServer.DownloadPublished("/d/", func(upath string, resp *rpc_astore.RetrieveResponse, err error, w http.ResponseWriter, r *http.Request) {
			DownloadHandler(listURL, upath, resp, err, w, r)
		}, w, r)
	})
	server.RegisterAssets(&stats, assets.Data, "", server.BasicMapper(server.MuxMapper(mux)))
	server.RegisterAssets(&stats, configs.Data, "", server.PrefixMapper("/configs", server.StripExtensionMapper(server.BasicMapper(server.MuxMapper(mux)))))
	stats.Log(log.Printf)

	mux.HandleFunc("/w", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && ((cookieDomain != "" && strings.Index(origin, cookieDomain) >= 0)) {
			w.Header().Add("Vary", "Origin")
			w.Header().Add("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Access-Control-Allow-Origin", origin)
		}

		redirect := r.URL.Query().Get("r")
		mods := []oauth.LoginModifier{oauth.WithCookieOptions(kcookie.WithPath("/"))}
		if redirect != "" {
			target, err := url.Parse(redirect)
			// This allows redirects to any machine that will accept the authentication cookie, and chrome extensions.
			if err == nil && ((cookieDomain != "" && strings.HasSuffix(target.Hostname(), cookieDomain)) || strings.HasPrefix(target.Scheme, "chrome")) {
				mods = append(mods, oauth.WithTarget(redirect))
			}
		}

		err := authWeb.PerformLogin(w, r, mods...)
		if err != nil {
			ShowResult(w, r, "broken", "Something Went Wrong", messageError, http.StatusUnauthorized)
			return
		}
	})

	// Path /a/ is used for CLI authentication. URL contains a key used by the CLI tool.
	mux.HandleFunc("/a/", func(w http.ResponseWriter, r *http.Request) {
		key, err := common.KeyFromURL(r.URL.Path)
		if err != nil {
			http.Error(w, "invalid authorization path, tough luck, try again", http.StatusUnauthorized)
			return
		}
		if err := authWeb.PerformLogin(w, r, oauth.WithState(*key), oauth.WithCookieOptions(kcookie.WithPath("/"))); err != nil {
			http.Error(w, "oauth failed, no idea why, ask someone to look at the logs", http.StatusUnauthorized)
			log.Printf("ERROR - could not perform login - %s", err)
			return
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ShowResult(w, r, "angry", "Nothing to see here", messageNothing, http.StatusUnauthorized)
	})

	// Path /e/ is the landing page at the end of the oauth authentication.
	mux.HandleFunc("/e/", func(w http.ResponseWriter, r *http.Request) {
		copts := []kcookie.Modifier{kcookie.WithPath("/")}
		if cookieDomain != "" {
			// WithSecure and WithSameSite are required to get the cookie forwarded via the NASSH plugin in chrome (for SSH).
			copts = append(copts, kcookie.WithDomain(cookieDomain), kcookie.WithSecure(true), kcookie.WithSameSite(http.SameSiteNoneMode))
		}

		data, handled, err := authWeb.PerformAuth(w, r, copts...)
		if err != nil {
			ShowResult(w, r, "angry", "Not Authorized", messageFail, http.StatusUnauthorized)
			log.Printf("ERROR - could not perform login - %s", err)
			return
		}

		if key, ok := data.State.(common.Key); ok {
			authServer.FeedToken(key, data.Cookie)
		}
		if !handled {
			ShowResult(w, r, "thumbs-up", "Good Job!", messageSuccess, http.StatusOK)
		}
	})

	server.Run(mux, grpcs)
	return nil
}

func main() {
	command := &cobra.Command{
		Use:   "astore-server",
		Short: "astore-server is an artifact and authentication server, usable as the backend for the 'astore' CLI tool",
	}

	astoreFlags := astore.DefaultFlags().Register(&kcobra.FlagSet{command.Flags()}, "")
	authFlags := auth.DefaultFlags().Register(&kcobra.FlagSet{command.Flags()}, "")
	oauthFlags := oauth.DefaultFlags().Register(&kcobra.FlagSet{command.Flags()}, "")

	targetURL := ""
	cookieDomain := ""
	command.Flags().StringVar(&targetURL, "site-url", "", "The URL external users can use to reach this web server")
	command.Flags().StringVar(&cookieDomain, "cookie-domain", "", "The domain for which the issued authentication cookie is valid. "+
		"This implicitly authorizes redirection to any URL within the domain.")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return Start(targetURL, cookieDomain, astoreFlags, authFlags, oauthFlags)
	}

	kcobra.PopulateDefaults(command, os.Args,
		kflags.NewAssetResolver(&logger.NilLogger{}, "astore-server", credentials.Data),
	)
	kcobra.Run(command)
}
