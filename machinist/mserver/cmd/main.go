package main

import (
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/machinist/mserver"
	"github.com/enfabrica/enkit/machinist/server/flags"
	"github.com/spf13/cobra"
	"math/rand"
	"net"
	"os"
	"time"
)

func Start(oauthFlags *oauth.RedirectorFlags) error {
	rng := rand.New(srand.Source)
	credMod := mserver.WithGenerateNewCredentials(
		kcerts.WithCountries([]string{"US"}),
		kcerts.WithValidUntil(time.Now().AddDate(3, 0, 0)),
		kcerts.WithNotValidBefore(time.Now().Add(-4*time.Minute)),
		kcerts.WithIpAddresses([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")}))
	//
	enc, err := token.NewSymmetricEncoder(rng, token.WithGeneratedSymmetricKey(0))
	if err != nil {
		return err
	}
	s, err := mserver.New(credMod, mserver.WithEncoder(enc), mserver.WithPort(8080))
	if err != nil {
		return err
	}
	return s.Start()
}

func main() {
	command := &cobra.Command{
		Use:   "controller",
		Short: "controller is a server in charge of controlling workers",
	}

	oauthFlags := oauth.DefaultRedirectorFlags()
	oauthFlags.Register(&kcobra.FlagSet{command.Flags()}, "")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return Start(oauthFlags)
	}

	kcobra.PopulateDefaults(command, os.Args,
		kflags.NewAssetAugmenter(&logger.NilLogger{}, "controller", flags.Data),
	)
	kcobra.Run(command)
}
