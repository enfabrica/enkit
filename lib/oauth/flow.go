package oauth

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/token"
	"golang.org/x/oauth2"
	"reflect"
)

type FlowController struct {
	required     *oauth2.Config
	optional     *oauth2.Config
	Enc          *token.TypeEncoder
}

// MarkAsDone will tell the flow that the oauth2.Config has been redeemed for this Identity. The next oauth2.Config
// fetched from FetchOauthConfig will be different. It returns back the modified state to be used in the next oauth2 flow.
func (fc *FlowController) MarkAsDone(state interface{}, conf *oauth2.Config, identity Identity) (*FlowState, error) {
	flowState, err := fc.DecodeState(state)
	if err != nil {
		return nil, err
	}
	if conf == fc.optional {
		flowState.OptionalDone = true
		flowState.Identities = append(flowState.Identities, identity)
	}
	if conf == fc.required {
		flowState.RequiredDone = true
		flowState.PrimaryIdentity = identity
	}
	return flowState, nil
}

// ShouldRedirect tells the server if it should redirect or not. It will always return false if no optional flows are present,
// otherwise it will return back if the flow is complete.
func (fc *FlowController) ShouldRedirect(state interface{}) (bool, error) {
	fs, err := fc.DecodeState(state)
	if err != nil {
		return false, err
	}
	if fc.optional == nil {
		return false, nil
	}
	if fs.OptionalDone && fs.RequiredDone {
		return false, err
	}
	return true, nil
}

// Identities will return the primary identity and a list of the optional flow identities redeemed.
func (fc *FlowController) Identities(state interface{}) (Identity, []Identity, error) {
	flowState, err := fc.DecodeState(state)
	if err != nil {
		return Identity{}, nil, err
	}
	return flowState.PrimaryIdentity, flowState.Identities, nil
}

type FlowState struct {
	OptionalDone    bool
	RequiredDone    bool
	Identities      []Identity
	PrimaryIdentity Identity
	Extra           interface{}
}

func (fc *FlowController) NewState(Extra interface{}) *FlowState {
	return &FlowState{
		OptionalDone: false,
		RequiredDone: false,
		Extra:        Extra,
	}
}

var errStateNotType = errors.New("state did not match")

func (fc *FlowController) FetchOauth2Config(state interface{}) (*oauth2.Config, error) {
	fs, err := fc.DecodeState(state)
	if err != nil {
		return nil, err
	}
	if fc.optional == nil || fs.OptionalDone {
		return fc.required, nil
	}
	return fc.optional, nil
}

func (fc *FlowController) DecodeState(state interface{}) (*FlowState, error) {
	if fs, ok := state.(*FlowState); ok {
		return fs, nil
	} else if fString, ok := state.(string); ok {
		var flows FlowState
		if _, err := fc.Enc.Decode(context.Background(), []byte(fString), &flows); err != nil {
			return nil, fmt.Errorf("%v, was %s expected %s, err %v", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&FlowState{}), err)
		}
		return &flows, nil
	} else if fs, ok := state.(FlowState); ok {
		return &fs, nil
	}
	return nil, fmt.Errorf("%v, was %s expected %s", errStateNotType, reflect.TypeOf(state), reflect.TypeOf(&FlowState{}))
}

func init() {
	gob.Register(FlowState{})
}
