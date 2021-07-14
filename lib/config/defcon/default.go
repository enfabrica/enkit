// Returns the default config store used in the enkit repository.
package defcon

import (
	"github.com/enfabrica/enkit/lib/config"
	"github.com/enfabrica/enkit/lib/config/directory"
)

func Open(app string, namespace ...string) (config.Store, error) {
	store, err := directory.OpenHomeDir(app, namespace...)
	if err != nil {
		return nil, err
	}
	return config.NewMulti(store), nil
}
