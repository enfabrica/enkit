package astore

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/enfabrica/enkit/lib/errdiff"
)

// modify returns a copy of `c` with entries in `mods` applied to it.
func modify(c jwt.MapClaims, mods map[string]any) jwt.MapClaims {
	newClaims := jwt.MapClaims{}
	for k, v := range c {
		newClaims[k] = v
	}
	for k, v := range mods {
		newClaims[k] = v
	}
	return newClaims
}

func TestValidateToken(t *testing.T) {
	privateKey := generateTokenKeypair(t)

	validToken := jwt.MapClaims{
		"sub": "example@customers.enfabrica.net",
		"nbf": time.Now().Unix(),
		"exp": time.Now().Add(8 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"uid": "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
	}

	testCases := []struct {
		desc    string
		claims  jwt.MapClaims
		uid     string
		wantErr string
	}{
		{
			desc:   "valid token",
			claims: validToken,
			uid:    "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
		},
		{
			desc:    "uid mismatch",
			claims:  validToken,
			uid:     "y27wi6och3foxew35gcrv34n4twnwx4i",
			wantErr: "UID requested does not match token",
		},
		{
			desc: "token not yet valid",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(1 * time.Hour).Unix(),
			}),
			uid:     "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
			wantErr: "token is not valid yet",
		},
		{
			desc: "token expired",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(-8 * time.Hour).Unix(),
				"exp": time.Now().Add(-1 * time.Hour).Unix(),
				"iat": time.Now().Add(-8 * time.Hour).Unix(),
			}),
			uid:     "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
			wantErr: "token is expired",
		},
		{
			desc: "token issue inside slack",
			claims: modify(validToken, map[string]any{
				"iat": time.Now().Add(9 * time.Minute).Unix(),
			}),
			uid: "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
		},
		{
			desc: "token issue outside slack",
			claims: modify(validToken, map[string]any{
				"iat": time.Now().Add(11 * time.Minute).Unix(),
			}),
			uid:     "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
			wantErr: "token used before issued",
		},
		{
			desc: "token notbefore inside slack",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(9 * time.Minute).Unix(),
			}),
			uid: "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
		},
		{
			desc: "token notbefore outside slack",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(11 * time.Minute).Unix(),
			}),
			uid:     "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
			wantErr: "token is not valid yet",
		},
		{
			desc: "token expiry inside slack",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(-8 * time.Hour).Unix(),
				"exp": time.Now().Add(-9 * time.Minute).Unix(),
				"iat": time.Now().Add(-8 * time.Hour).Unix(),
			}),
			uid: "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
		},
		{
			desc: "token expiry outside slack",
			claims: modify(validToken, map[string]any{
				"nbf": time.Now().Add(-8 * time.Hour).Unix(),
				"exp": time.Now().Add(-11 * time.Minute).Unix(),
				"iat": time.Now().Add(-8 * time.Hour).Unix(),
			}),
			uid:     "akr2jbxff7fqhzqv3xy7faquzn2j8u56",
			wantErr: "token is expired",
		},
	}
	for _, tc := range testCases {
		srv, _ := serverForTest()
		srv.options.tokenPublicKeys = []jwt.VerificationKey{&privateKey.PublicKey}

		t.Run(tc.desc, func(t *testing.T) {
			tokenPayload := createToken(t, privateKey, tc.claims)
			gotErr := srv.validateToken(tokenPayload, tc.uid)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
		})
	}
}
