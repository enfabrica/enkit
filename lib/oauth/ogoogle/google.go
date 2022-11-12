package ogoogle

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-oidc"
	"github.com/enfabrica/enkit/lib/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
	"io/ioutil"
	"strings"
)

func Defaults() oauth.Modifier {
	return oauth.WithModifiers(
		oauth.WithEndpoint(google.Endpoint),
		oauth.WithFactory(NewOidJWTVerifier),
	)
}

func SplitUsername(email, hd string) (string, string) {
	index := strings.Index(strings.TrimSpace(email), "@")
	if index >= 0 {
		return email[:index], email[index+1:]
	}
	return email, hd
}

// GetGroupsVerifier retrieves the user membership in groups.
//
// The groups are stored in the identity cookie of the user, and can optionally
// be used to deny (or allow) access.
//
// The GetGroupsVerifier relies on the cloudidentity API, which is very poorly
// documented here:
//   https://pkg.go.dev/google.golang.org/api/cloudidentity/v1beta1#GroupsMembershipsService.SearchTransitiveGroups
//
// ... and was announced in this blog post:
//   https://workspaceupdates.googleblog.com/2020/08/new-api-cloud-identity-groups-google.html
//
// The API is the only "documented" API that allows to retrieve membership
// without having an admin key / admin privileges - relying solely on the
// credentials of the user.
//
// The API is recursive/transitive: if user is member of group A, and group A is
// member of group B, then the user is member of both A and B. However, this API
// has one major flaw: if one of the groups is closed/configured such as the API
// cannot compute the transitive membership, the API fails entirely returning
// an error.
//
// If any user of your org is part of an external/public google group, the API
// will most likely fail for that user.
//
// The API allows specifying a query, but the the set of fields available
// for the query seems undocumented? We could not find any way to constrain the
// recursive search to only groups belonging to the domain.
//
// However, the query can be constrained to the "kind of group" used.
// Specifically, it recognizes two kinds of groups:
// - traditional google groups (label "groups.discussion_forum")
// - security groups (label "groups.security")
// - ... a few more not relevant ...
//
// From documentation, any traditional google group can be labeled to be
// a security group. Once labeled, a few more restrictions are enforced,
// including preventing external or non-security groups from joining.
//
// GetGroupsVerifier will thus only return security groups.
//
// To label a group as a security group, you can tag it via UI by visiting
// admin.google.com (groups.google.com does not expose the feature!) or by
// using `gcloud identity groups update --labels=...`. More details here:
//		https://support.google.com/a/answer/10607394?hl=en
type GetGroupsVerifier struct {
	conf *oauth2.Config
}

func (ggv *GetGroupsVerifier) Scopes() []string {
	return []string{
		"https://www.googleapis.com/auth/cloud-identity.groups.readonly",
	}
}

func (gui *GetGroupsVerifier) Verify(identity *oauth.Identity, tok *oauth2.Token) (*oauth.Identity, error) {
	if identity == nil {
		return nil, fmt.Errorf("group verifier can only be run after another verifier established identity")
	}

	if identity.Username == "" || identity.Organization == "" {
		return nil, fmt.Errorf("invalid empty Username or Organization supplied to group verifier")
	}

	email := identity.GlobalName()
	// See below, defense in depth against any sort of query injection.
	if strings.Contains(email, "'") {
		return nil, fmt.Errorf("invalid email contains unsafe characters - %s", email)
	}

	cis, err := cloudidentity.NewService(context.Background(),
		option.WithTokenSource(gui.conf.TokenSource(context.Background(), tok)))
	if err != nil {
		return nil, fmt.Errorf("for user %s cloudidentity refused token - %w", email, err)
	}

	// TODO(carlo): the fmt.Sprintf() makes me extremely uncomfortable. SQL injection mumble mumble.
	//   We verify the string does not contain a ' just a few lines above, so it "should" be safe.
	//   But... are there other characters that should be escaped? Not clear from all we know about
	//   the CEL language, and could not find a library to safely compose queries or escape fields!
	//   Other projects seem not to care at all:
	//	https://github.com/gravitational/teleport/blob/0ee91f6c37eb3a8c5bc98b15f857c95902df50b2/lib/auth/oidc_google.go#L238
	//	https://github.com/salrashid123/opa_external_groups/blob/226989f313306c2435896a8095362207b4641cc2/groups_server/server.go#L58
	//
	// To query traditional groups, use:
	//	"member_key_id=='%s' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels"
	search := cloudidentity.NewGroupsMembershipsService(cis).SearchTransitiveGroups("groups/-").Query(
		fmt.Sprintf(
			"member_key_id=='%s' && 'cloudidentity.googleapis.com/groups.security' in labels",
			email),
	)

	groups := []string{}
	if err := search.Pages(context.TODO(), func(page *cloudidentity.SearchTransitiveGroupsResponse) error {
		// Example response in yaml (gcloud command):
		// - displayName: gcp-group
		//   group: groups/0xx123x011xxxx
		//   groupKey:
		//     id: gcp-group@enfabrica.net
		//   labels:
		//     cloudidentity.googleapis.com/groups.discussion_forum: ''
		//     cloudidentity.googleapis.com/groups.security: ''
		//   relationType: DIRECT
		//   roles:
		//   - role: OWNER
		//   - role: MEMBER
		for _, m := range page.Memberships {
			groups = append(groups, m.GroupKey.Id)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("transitive search for %s returned - %w", email, err)
	}

	identity.Groups = append(identity.Groups, groups...)
	return identity, nil
}

type GetUserInfoVerifier struct {
	conf *oauth2.Config
}

func (gui *GetUserInfoVerifier) Scopes() []string {
	return []string{
		"https://www.googleapis.com/auth/userinfo.email",
	}
}

func (gui *GetUserInfoVerifier) Verify(identity *oauth.Identity, tok *oauth2.Token) (*oauth.Identity, error) {
	// FIXME: timeout, retry strategy.
	client := gui.conf.Client(oauth2.NoContext, tok)
	email, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("token did not give access to userinfo - %w", err)
	}
	defer email.Body.Close()
	data, _ := ioutil.ReadAll(email.Body)

	var userinfo struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Hd    string `json:"hd"`
	}
	if err := json.Unmarshal(data, &userinfo); err != nil {
		return nil, fmt.Errorf("could not decode json %s - %w", string(data), err)
	}

	identity.Id = "google:" + userinfo.Hd + ":" + userinfo.Sub
	identity.Username, identity.Organization = SplitUsername(userinfo.Email, userinfo.Hd)
	return identity, nil
}

// GetUserInfoVerifier tries to fetch the userinfo of a user to verify the validity of a token.
//
// It performs an http request for every attempt to validate the token. If the request fails,
// either the token is invalid, or there is a problem with the API backend.
func NewGetUserInfoVerifier(conf *oauth2.Config) (oauth.Verifier, error) {
	return &GetUserInfoVerifier{conf: conf}, nil
}

type OidJWTVerifier struct {
	overifier *oidc.IDTokenVerifier
}

func (ojt *OidJWTVerifier) Scopes() []string {
	return []string{
		"https://www.googleapis.com/auth/userinfo.email",
	}
}

func (ojt *OidJWTVerifier) Verify(identity *oauth.Identity, tok *oauth2.Token) (*oauth.Identity, error) {
	// TODO: oid parse jwt token to avoid the call to googleapis below here.
	// https://github.com/coreos/go-oidc
	// Extract the ID Token from OAuth2 token.
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("id_token parameter not supplied")
	}

	// Parse and verify ID Token payload.
	idToken, err := ojt.overifier.Verify(context.TODO(), rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verification of id_token failed - %w", err)
	}

	var claim struct {
		Email string `json:"email"`
		Hd    string `json:"hd"`
		Sub   string `json:"sub"`
	}
	if err := idToken.Claims(&claim); err != nil {
		return nil, fmt.Errorf("idtoken did not contain necessary claims - %w", err)
	}

	identity.Id = "google:" + claim.Hd + ":" + claim.Sub
	identity.Username, identity.Organization = SplitUsername(claim.Email, claim.Hd)
	return identity, nil
}

// OidJWTVerifier fetches a google certificate over https once, and uses it to verify the
// signature in the JWT extra information attached to a returned token.
//
// This only requires fetching a certificate at startup (and well... ideally, refreshing it
// every now and then), to then use simple crypto functions to verify the singature on every token.
func NewOidJWTVerifier(conf *oauth2.Config) (oauth.Verifier, error) {
	// FIXME: retry logic, timeout, http failure handling.
	provider, err := oidc.NewProvider(context.TODO(), "https://accounts.google.com")
	if err != nil {
		return nil, err
	}

	if conf.ClientID == "" {
		return nil, fmt.Errorf("API usage error - OidJWTVerifier factory can only be used after Secrets loaded - after With.*Secrets")
	}

	return &OidJWTVerifier{
		overifier: provider.Verifier(&oidc.Config{ClientID: conf.ClientID}),
	}, nil
}
