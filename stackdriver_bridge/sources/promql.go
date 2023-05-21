package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	promapicommon "github.com/prometheus/client_golang/api"
	promapi "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
)

type PromQL interface {
	Query(context.Context, string, time.Time, ...promapi.Option) (model.Value, promapi.Warnings, error)
}

func OpenPromQL(endpoint string) (PromQL, error) {
	cj, err := kcookie.NewJar()
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Jar: cj,
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %q: %w", err)
	}

	if parsed.Scheme == "https" {
		glog.Infof("Using local enkit credentials for external endpoint %q", endpoint)
		store, err := identity.NewStore("enkit", defcon.Open)
		if err != nil {
			return nil, fmt.Errorf("failed to open store while loading local creds: %w", err)
		}
		_, token, err := store.Load("")
		if err != nil {
			return nil, fmt.Errorf("failed to load token while loading local creds: %w", err)
		}
		cookie := kcookie.New("Creds", token)
		httpClient.Jar.SetCookies(parsed, []*http.Cookie{cookie})
	}

	if glog.V(3) {
		glog.Info("HTTP request/response logging enabled")
		httpClient.Transport = &khttp.LoggingTransport{
			Transport: http.DefaultTransport,
			Log:       glog.V(3).Infof,
		}
	}

	client, err := promapicommon.NewClient(promapicommon.Config{
		Address: endpoint,
		Client:  httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open PromQL endpoint %q: %w", err)
	}

	return promapi.NewAPI(client), nil
}
