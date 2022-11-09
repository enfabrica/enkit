package ogoogle

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-oidc"
	"github.com/enfabrica/enkit/lib/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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
