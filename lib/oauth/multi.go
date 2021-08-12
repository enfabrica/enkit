package oauth

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/token"
	"math/rand"
	"net/http"
	"reflect"
)

const requiredKey = "required"

type MultiOauth struct {
	RequiredAuth *Authenticator
	OptAuth      map[string]*Authenticator
	Enc          []*token.TypeEncoder
}

type MultiOAuthState struct {
	CompletedFlows []string
	CurrentFlow    string
	OptIdentities  []Identity
	Extra          interface {
	}
}

func (mo *MultiOauth) NewState(Extra interface{}) *MultiOAuthState {
	return &MultiOAuthState{
		CompletedFlows: []string{},
		Extra:          Extra,
	}
}

var errStateNotType = errors.New("state did not match")

func (mo *MultiOauth) decodeRaw(state interface{}) (*MultiOAuthState, error) {
	if fs, ok := state.(*MultiOAuthState); ok {
		return fs, nil
	} else if fString, ok := state.(string); ok {
		var errs []error
		for _, enc := range mo.Enc {
			var loginState LoginState
			if _, err := enc.Decode(context.Background(), []byte(fString), &loginState); err != nil {
				errs = append(errs, fmt.Errorf("%v, was %s expected %s, err %v", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&MultiOAuthState{}), err))
			}
			if s, ok := loginState.State.(MultiOAuthState); ok {
				return &s, nil
			}
		}
		return nil, multierror.New(errs)
	} else if fs, ok := state.(MultiOAuthState); ok {
		return &fs, nil
	}
	return nil, fmt.Errorf("%v, was %s expected %s", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&MultiOAuthState{}))
}

// decodeState will check if the state alignment is present. If it isnt it will return a
func (mo *MultiOauth) decodeState(r *http.Request) (*MultiOAuthState, error) {
	query := r.URL.Query()
	state := query.Get("state")
	// No state means fresh flow
	if state == "" {
		if mo.OptAuth == nil {
			return &MultiOAuthState{}, nil
		}
		return &MultiOAuthState{}, nil
	}
	return mo.decodeRaw(state)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// authenticator will return the specified oauth.Authenticator if MultiOAuthState.CurrentFlow is set.
// if it is not, it will spit our a random optional flow or the last, required flow. It also will set MultiOAuthState.CurrentFlow.
func (mo *MultiOauth) authenticator(state *MultiOAuthState) (string, *Authenticator) {
	if state.CurrentFlow != "" {
		return state.CurrentFlow, mo.OptAuth[state.CurrentFlow]
	}
	// find the first authen not redeemed, if they are all redeemed return required
	for k, v := range mo.OptAuth {
		if !contains(state.CompletedFlows, k) {
			state.CurrentFlow = k
			return k, v
		}
	}
	state.CurrentFlow = requiredKey
	return requiredKey, mo.RequiredAuth
}

func (mo *MultiOauth) PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, bool, error) {
	flowState, err := mo.decodeState(r)
	if err != nil {
		return AuthData{}, false, err
	}
	key, a := mo.authenticator(flowState)
	data, _, err := a.PerformAuth(w, r, mods...)
	if err != nil {
		return AuthData{}, false, err
	}
	// This is the terminal condition.
	if key == requiredKey {
		data.PrimaryIdentity = data.Creds.Identity
		data.Identities = flowState.OptIdentities
		data.State = flowState.Extra
		return data, true, err
	}
	flowState.OptIdentities = append(flowState.OptIdentities, data.Creds.Identity)
	flowState.CompletedFlows = append(flowState.CompletedFlows, key)
	flowState.CurrentFlow = ""
	data.State = flowState
	return data, false, err
}

func (mo *MultiOauth) PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error {
	options := LoginModifiers(lm).Apply(&LoginOptions{})
	// if the passed in state does not match what we want, pack in the previous state to unpack later.
	if _, ok := options.State.(*MultiOAuthState); !ok {
		options.State = mo.NewState(options.State)
	}
	// if the passed in options arent already a flow state, pack them up to unpack later.
	state, err := mo.decodeRaw(options.State)
	if err != nil {
		return err
	}
	_, a := mo.authenticator(state)

	// override just in case we had to pack it in, login apply works in FIFO
	lm = append(lm, WithState(options.State))
	return a.PerformLogin(w, r, append(lm)...)
}

func (mo MultiOauth) WithCredentialsOrError(handler khttp.FuncHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		handler(writer, request)
	}
}

func init() {
	gob.Register(MultiOAuthState{})
}

// I want uuids
var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(rng *rand.Rand, n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func NewMultiOAuth(rng *rand.Rand, required *Authenticator, opts ...*Authenticator) *MultiOauth {
	var encs []*token.TypeEncoder
	m := map[string]*Authenticator{}
	for _, v := range opts {
		encs = append(encs, v.authEncoder)
		m[randSeq(rng, 25)] = v
	}

	m[requiredKey] = required
	encs = append(encs, required.authEncoder)

	return &MultiOauth{
		RequiredAuth: required,
		OptAuth:      m,
		Enc:          encs,
	}
}
