package plugins

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// GoogleGroupsPlugin can only be enabled if the primary identity's oauth2 provider is google. It will embed the google groups
// in the extensions of the
type GoogleGroupsPlugin struct {
	Config *oauth2.Config
}

func (g *GoogleGroupsPlugin) CertMods(ctx context.Context, creds oauth.AuthData) ([]kcerts.CertMod, error) {
	srv, err := admin.NewService(ctx, option.WithHTTPClient(g.Config.Client(ctx, &creds.Creds.Token)))
	if err != nil {
		fmt.Println("error creating service", err.Error())
		return nil, err
	}
	r, err := srv.Groups.List().Do()
	if err != nil {
		fmt.Println("error fetching groups", err.Error())
		return nil, err
	}
	var mods []kcerts.CertMod
	for index, g := range r.Groups {
		k := fmt.Sprintf("google-group-%d", index)
		mods = append(mods, kcerts.AddExtensionMod(k, g.Name))
	}
	return mods, nil
}

func (g *GoogleGroupsPlugin) Init() error {
	return nil
}

func (g *GoogleGroupsPlugin) AddFlags(command *cobra.Command) *cobra.Command {
	return command
}
