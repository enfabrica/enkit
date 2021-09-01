package enauth

import (
	"context"
	"github.com/enfabrica/enkit/enauth/plugins"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/spf13/cobra"
)

type Plugin interface {
	Init() error
	AddFlags(command *cobra.Command) *cobra.Command
	CertMods(ctx context.Context, creds oauth.AuthData) ([]kcerts.CertMod, error)
}

var _ Plugin = &plugins.GoogleGroupsPlugin{}
