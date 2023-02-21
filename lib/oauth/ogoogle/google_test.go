package ogoogle

import (
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"testing"
)

func TestGroups(t *testing.T) {
	testFactory := func(conf *oauth2.Config) (oauth.Verifier, error) {
		v, err := NewGetGroupsVerifier(conf)
		if err != nil {
			return v, err
		}

		v.(*GetGroupsVerifier).BasePath = "https://non-existant-domain.lan./"
		return v, err
	}

	// Ensure that the test factory can instantiate a valid factory...
	v, err := testFactory(nil)
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.True(t, len(v.Scopes()) > 0)

	// ... that fails the query / returns an error.
	res, err := v.Verify(logger.Go, &oauth.Identity{Username: "test", Organization: "tester.org"}, nil)
	assert.Nil(t, res)
	assert.Error(t, err)

	acc := logger.NewAccumulator()

	// Let's wrap the failing verifier around an optional verifier...
	of := oauth.NewOptionalVerifierFactory(testFactory)
	v, err = of(nil)
	assert.NoError(t, err)
	assert.NotNil(t, v)

	// ... and check that it no longer fails verification.
	res, err = v.Verify(acc, &oauth.Identity{Username: "test", Organization: "tester.org"}, nil)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// ... but a message is logged nontheless.
	logs := acc.Retrieve()
	assert.True(t, len(logs) == 1)
	assert.Regexp(t, `test@tester.org.*GetGroupsVerifier.*non-existant-domain.lan.`, logs[0])
}
