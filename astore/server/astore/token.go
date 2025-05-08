package astore

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrTokenUidMismatch = errors.New("UID requested does not match token")

type claims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	Expiry    int64  `json:"exp"`
	NotBefore int64  `json:"nbf"`
	IssuedAt  int64  `json:"iat"`
	AstoreUid string `json:"uid"`

	requestedUid string
}

func (c claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.Expiry, 0)), nil
}

func (c claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

func (c claims) GetNotBefore() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.NotBefore, 0)), nil
}

func (c claims) GetIssuer() (string, error) {
	return c.Issuer, nil
}

func (c claims) GetSubject() (string, error) {
	return c.Subject, nil
}

func (c claims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings([]string{c.Audience}), nil
}

func (c claims) Validate() error {
	if c.AstoreUid != c.requestedUid {
		return ErrTokenUidMismatch
	}
	return nil
}

func (s *Server) validateToken(token string, uid string) error {
	parser := jwt.NewParser(jwt.WithIssuedAt(), jwt.WithLeeway(10*time.Minute))
	c := claims{requestedUid: uid}
	_, err := parser.ParseWithClaims(token, &c, s.tokenValidationKey)
	if err != nil {
		return fmt.Errorf("token failed to parse/verify: %w", err)
	}
	return nil
}

func (s *Server) tokenValidationKey(t *jwt.Token) (any, error) {
	if len(s.options.tokenPublicKeys) == 0 {
		return nil, fmt.Errorf("server is not configured to validate token URL params")
	}
	return jwt.VerificationKeySet{Keys: s.options.tokenPublicKeys}, nil
}
