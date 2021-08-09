package oauth

import (
	"encoding/hex"
	"fmt"
	"github.com/enfabrica/enkit/astore/common"
	"golang.org/x/oauth2"
	"sync"
)

type FlowState struct {
	OptionalUsed    bool
	RequiredUsed    bool
	Identities      []Identity
	PrimaryIdentity Identity
}

type FlowController struct {
	currentFlows map[string]*FlowState
	flowLock     sync.RWMutex
	required     *oauth2.Config
	optional     *oauth2.Config
}

func (fc *FlowController) getState(keyID *common.Key) (*FlowState, error) {
	flowID := hex.EncodeToString(keyID[:])
	fc.flowLock.RLock()
	defer fc.flowLock.RUnlock()
	state := fc.currentFlows[flowID]
	if state == nil {
		return nil, fmt.Errorf("flow %s id not exist", flowID)
	}
	return state, nil
}

func (fc *FlowController) saveState(keyID *common.Key, state *FlowState) {
	fc.flowLock.Lock()
	defer fc.flowLock.Unlock()
	flowID := hex.EncodeToString(keyID[:])
	fc.currentFlows[flowID] = state
}

// FirstOrCreateFlow
func (fc *FlowController) FirstOrCreateFlow(keyID *common.Key) {
	_, err := fc.getState(keyID)
	if err != nil {
		fc.saveState(keyID, &FlowState{})
	}
}

func (fc *FlowController) FetchOauthConfig(keyID *common.Key) (*oauth2.Config, error) {
	flowState, err := fc.getState(keyID)
	if err != nil {
		return nil, err
	}
	if flowState.OptionalUsed || fc.optional == nil {
		return fc.required, nil
	}
	return fc.optional, nil
}

func (fc *FlowController) MarkAsDone(keyID *common.Key, conf *oauth2.Config, identity Identity) error {
	flowState, err := fc.getState(keyID)
	if err != nil {
		return err
	}
	if conf == fc.optional {
		flowState.OptionalUsed = true
		flowState.Identities = append(flowState.Identities, identity)
	}
	if conf == fc.required {
		flowState.RequiredUsed = true
		flowState.PrimaryIdentity = identity
	}
	fc.saveState(keyID, flowState)
	return nil
}
// ShouldRedirect tells the server if it should redirect or not. It will always return false if no optional flows are present,
// otherwise it will return back if the flow is complete.
func (fc *FlowController) ShouldRedirect(keyID *common.Key) bool {
	flowState, err := fc.getState(keyID)
	if err != nil {
		return false
	}
	if flowState.OptionalUsed && flowState.RequiredUsed {
		return false
	}
	if fc.optional == nil {
		return false
	}
	return true
}

func (fc *FlowController) Identities(keyID *common.Key) (Identity, []Identity, error) {
	flowState, err := fc.getState(keyID)
	if err != nil {
		return Identity{}, nil, err
	}
	return flowState.PrimaryIdentity, flowState.Identities, nil
}
