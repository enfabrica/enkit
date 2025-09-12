package astore

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestACLBasics(t *testing.T) {
	acl, err := NewACLList(nil)
	assert.Equal(t, 0, len(acl))
	assert.NoError(t, err)
	assert.NoError(t, acl.IsAllowed(nil))
	assert.NoError(t, acl.IsUserAllowed("whatever"))
}

func TestACLParsing(t *testing.T) {
	// No ACLs defaults to allowing anything.
	acl, err := NewACLList([]string{})
	assert.Equal(t, 0, len(acl))
	assert.NoError(t, err)
	assert.NoError(t, acl.IsAllowed(nil))
	assert.NoError(t, acl.IsUserAllowed("whatever"))

	// An invalid ACL (no separator), should result in an error.
	acl, err = NewACLList([]string{"+:.*", "invalid"})
	assert.ErrorContains(t, err, "ACL#1:")

	// Another invalid ACL (invalid action).
	acl, err = NewACLList([]string{"+:.*", "f:.*"})
	assert.ErrorContains(t, err, "ACL#1:")

	// Another invalid ACL (invalid regexp).
	acl, err = NewACLList([]string{"+:.*", "+:.*[a"})
	assert.ErrorContains(t, err, "ACL#1:")

	// Finally a reasonable ACL.
	acl, err = NewACLList([]string{"-:.*@lic\\.enfabrica\\.net", "+:.*@enfabrica\\.net"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(acl))
	assert.NoError(t, acl.IsUserAllowed("whatever@enfabrica.net"))
	assert.ErrorContains(t, acl.IsUserAllowed("whatever@lic.enfabrica.net"), "denying")
	assert.ErrorContains(t, acl.IsUserAllowed("mario@bros.net"), "No ACL")
}
