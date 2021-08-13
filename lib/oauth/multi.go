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
	"log"
	"math/rand"
	"net/http"
	"reflect"
)

type MultiOauth struct {
	RequiredAuth *Authenticator
	OptAuth      []*Authenticator
	Enc          []*token.TypeEncoder
}

type MultiOAuthState struct {
	CurrentFlow   int
	OptIdentities []Identity
	Extra         interface{}
}

func (mo *MultiOauth) NewState(Extra interface{}) *MultiOAuthState {
	return &MultiOAuthState{
		Extra: Extra,
	}
}

var errStateNotType = errors.New("state did not match")

func (mo *MultiOauth) decodeRaw(state interface{}) (*MultiOAuthState, error) {
	if fs, ok := state.(*MultiOAuthState); ok {
		return fs, nil
	}
	if fs, ok := state.(MultiOAuthState); ok {
		return &fs, nil
	}
	if fString, ok := state.(string); ok {
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
	}
	return nil, fmt.Errorf("%v, was %s expected %s", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&MultiOAuthState{}))
}

// decodeState will check if the state alignment is present. If it isnt it will return a
func (mo *MultiOauth) decodeState(r *http.Request) (*MultiOAuthState, error) {
	query := r.URL.Query()
	state := query.Get("state")
	// No state means fresh flow
	if state == "" {
		return &MultiOAuthState{}, nil
	}
	return mo.decodeRaw(state)
}

// authenticator will return the specified oauth.Authenticator from MultiOAuthState.CurrentFlow.
// if the index matches the final optional authenticator, it will return the required one.
func (mo *MultiOauth) authenticator(state *MultiOAuthState) (int, *Authenticator) {
	if state.CurrentFlow >= len(mo.OptAuth) {
		return state.CurrentFlow, mo.RequiredAuth
	}
	// find the first authen not redeemed, if they are all redeemed return required
	return state.CurrentFlow, mo.OptAuth[state.CurrentFlow]
}

func (mo *MultiOauth) PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, bool, error) {
	flowState, err := mo.decodeState(r)
	if err != nil {
		return AuthData{}, true, err
	}
	_, a := mo.authenticator(flowState)
	data, _, err := a.PerformAuth(w, r, mods...)
	if err != nil {
		return AuthData{}, true, err
	}
	// This is the terminal condition.
	if a == mo.RequiredAuth {
		data.PrimaryIdentity = data.Creds.Identity
		data.Identities = flowState.OptIdentities
		data.State = flowState.Extra
		return data, false, err
	}
	flowState.OptIdentities = append(flowState.OptIdentities, data.Creds.Identity)
	flowState.CurrentFlow += 1
	data.State = flowState
	if err := mo.PerformLogin(w, r,
		WithState(data.State),
		WithCookieOptions(kcookie.WithPath("/")),
	); err != nil {
		http.Error(w, "oauth failed, no idea why, ask someone to look at the logs", http.StatusUnauthorized)
		log.Printf("ERROR - could not perform login - %s", err)
		return AuthData{}, true, err
	}
	return data, true, err
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

func NewMultiOAuth(rng *rand.Rand, required *Authenticator, opts ...*Authenticator) *MultiOauth {
	var encs []*token.TypeEncoder
	for _, v := range opts {
		encs = append(encs, v.authEncoder)
	}
	encs = append(encs, required.authEncoder)
	return &MultiOauth{
		RequiredAuth: required,
		OptAuth:      opts,
		Enc:          encs,
	}
}
