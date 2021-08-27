package authz_test

import (
	"context"
	"github.com/enfabrica/enkit/lib/authz"
	"github.com/enfabrica/enkit/lib/authz/plugins"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestAuthService_Do(t *testing.T) {
	// In the dummy service logic, all users own 1 resource, are an admin on that role,
	// and are user role on all other resources
	d := &plugins.DummyService{
		Users: []plugins.DummyUser{
			{
				Name: "groot",
				Owns: "groot-group",
				Root: true,
			},
			{
				Name: "baz",
				Owns: "baz-group",
			},
			{
				Name: "foo",
				Owns: "foo-group",
			},
			{
				Name: "bar",
				Owns: "bar-group",
			},
		},
	}
	s, err := authz.NewService(d)
	assert.Nil(t, err)

	// Groot can do anything
	err = s.NewRequest().
		OnResource("literally anything").
		WithAction(authz.ActionCreate).
		AsUser("groot").
		Verify(context.Background())
	assert.Nil(t, err)

	// Foo owns foo-group
	err = s.NewRequest().
		OnResource("foo-group").
		WithAction(authz.ActionCreate).
		AsUser("foo").
		Verify(context.Background())
	assert.Nil(t, err)

	// Bar does not own foo-group
	err = s.NewRequest().
		OnResource("foo-group").
		WithAction(authz.ActionCreate).
		AsUser("bar").
		Verify(context.Background())
	assert.NotNil(t, err)

	// bar can still read foo-group by design
	err = s.NewRequest().
		OnResource("foo-group").
		WithAction(authz.ActionRead).
		AsUser("bar").
		Verify(context.Background())
	assert.Nil(t, err)

}
