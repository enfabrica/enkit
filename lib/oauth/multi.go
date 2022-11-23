package oauth

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/lib/token"
	"log"
	"math/rand"
	"net/http"
	"reflect"
)

type MultiOauth struct {
	RequiredAuth   *Authenticator
	OptAuth        []*Authenticator
	Enc            []*token.TypeEncoder
	LoginModifiers []LoginModifier
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

func (mo *MultiOauth) decodeString(state string) (*MultiOAuthState, error) {
	var errs []error
	for _, enc := range mo.Enc {
		var loginState LoginState
		if _, err := enc.Decode(context.Background(), []byte(state), &loginState); err != nil {
			errs = append(errs, fmt.Errorf("%v, was %s expected %s, err %v", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&MultiOAuthState{}), err))
		}
		if s, ok := loginState.State.(MultiOAuthState); ok {
			return &s, nil
		}
	}
	return nil, multierror.New(errs)
}

// decodeState will check if the state alignment is present. If it isnt it will return a
func (mo *MultiOauth) decodeState(r *http.Request) (*MultiOAuthState, error) {
	query := r.URL.Query()
	state := query.Get("state")
	// No state means fresh flow
	if state == "" {
		return &MultiOAuthState{}, nil
	}
	return mo.decodeString(state)
}

// authenticator will return the specified oauth.Authenticator from MultiOAuthState.CurrentFlow.
// If the index matches the final optional authenticator, it will return the required one.
func (mo *MultiOauth) authenticator(state *MultiOAuthState) *Authenticator {
	if state.CurrentFlow >= len(mo.OptAuth) {
		return mo.RequiredAuth
	}
	return mo.OptAuth[state.CurrentFlow]
}

func (mo *MultiOauth) PerformAuth(w http.ResponseWriter, r *http.Request, mods ...kcookie.Modifier) (AuthData, error) {
	flowState, err := mo.decodeState(r)
	if err != nil {
		return AuthData{}, err
	}
	a := mo.authenticator(flowState)
	data, err := a.ExtractAuth(w, r)
	if err != nil {
		return AuthData{}, err
	}
	// This is the terminal condition.
	if a == mo.RequiredAuth {
		data, err := a.SetAuthCookie(data, w, mods...)
		if err != nil {
			return AuthData{}, err
		}

		data.Identities = flowState.OptIdentities
		data.State = flowState.Extra
		return data, nil
	}
	flowState.OptIdentities = append(flowState.OptIdentities, data.Creds.Identity)
	flowState.CurrentFlow += 1
	data.State = flowState
	if err := mo.PerformLogin(w, r, append(mo.LoginModifiers, WithState(data.State))...); err != nil {
		http.Error(w, "oauth failed, no idea why, ask someone to look at the logs", http.StatusUnauthorized)
		log.Printf("ERROR - could not perform login - %s", err)
		return AuthData{}, err
	}
	return data, nil
}

func (mo *MultiOauth) PerformLogin(w http.ResponseWriter, r *http.Request, lm ...LoginModifier) error {
	mo.LoginModifiers = lm
	options := LoginModifiers(lm).Apply(&LoginOptions{})

	// If the passed in state does not match what we want, pack in the previous state to unpack later.
	state, ok := options.State.(*MultiOAuthState)
	if !ok {
		state = mo.NewState(options.State)
		options.State = state
	}
	// If the passed in options are not already a flow state, pack them up to unpack later.
	a := mo.authenticator(state)
	// Override just in case we had to pack it in, login apply works in FIFO.
	lm = append(lm, WithState(options.State))
	return a.PerformLogin(w, r, lm...)
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
