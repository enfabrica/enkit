package astore

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/oauth"
	"regexp"
	"strings"
)

type ACLAction int

const (
	ACLUndefined ACLAction = iota
	ACLAllow
	ACLDeny
)

type ACL struct {
	action ACLAction
	match  *regexp.Regexp
}

type ACLList []ACL

func (a ACLList) IsAllowed(creds *oauth.CredentialsCookie) error {
	if creds == nil {
		if len(a) == 0 {
			return nil
		}

		return fmt.Errorf("no credentials provided in request - but ACLs are set, denying")
	}

	return a.IsUserAllowed(creds.Identity.GlobalName())
}

func (a ACLList) IsUserAllowed(user string) error {
	// If no ACL was configured at all, we allow the request, for backward compatibility.
	if len(a) == 0 {
		return nil
	}

	for ix, acl := range a {
		if acl.match.MatchString(user) {
			if acl.action == ACLAllow {
				return nil
			}

			if acl.action == ACLDeny {
				return fmt.Errorf("ACL#%d - matches user %s, denying access", ix, user)
			}
		}
	}
	return fmt.Errorf("No ACL matched user %s, denying access", user)
}

func NewACLList(aclsstr []string) (ACLList, error) {
	acls := ACLList{}
	for ix, acl := range aclsstr {
		splits := strings.SplitN(acl, ":", 2)
		if len(splits) != 2 {
			return nil, fmt.Errorf("ACL#%d: %s - is invalid - must be <action>:<regex>, no : separator found", ix, acl)
		}

		actionstr, restr := splits[0], splits[1]

		var action ACLAction
		switch actionstr {
		case "+":
			action = ACLAllow
		case "-":
			action = ACLDeny
		default:
			return nil, fmt.Errorf("ACL#%d: %s - is invalid - action must be + or -", ix, acl)
		}

		re, err := regexp.Compile(restr)
		if err != nil {
			return nil, fmt.Errorf("ACL#%d: %s - is invalid - invalid regex %s - %w", ix, acl, restr, err)
		}

		acls = append(acls, ACL{action, re})
	}

	return acls, nil
}
