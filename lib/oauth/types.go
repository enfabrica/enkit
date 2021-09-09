package oauth

import (
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"net/http"
)

type IAuthenticator interface {
	PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error
	PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, error)
	Complete(data AuthData) bool
}
