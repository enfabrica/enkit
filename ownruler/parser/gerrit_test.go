package parser

import (
	"github.com/enfabrica/enkit/lib/testutil"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestParseGerritOwners(t *testing.T) {
	pb, err := ParseGerritOwners("foo/bar", strings.NewReader(""))
	assert.NotNil(t, pb)
	assert.NoError(t, err)

	pb, err = ParseGerritOwners("foo/bar", strings.NewReader("set noparenting"))
	assert.Nil(t, pb)
	assert.Error(t, err)

	pb, err = ParseGerritOwners("foo/bar", strings.NewReader(`
fuffa # this is invalid - not a user, not a file.
*.txt barb
`))
	assert.Error(t, err)

	pb, err = ParseGerritOwners("foo/bar", strings.NewReader(`
# Simple Google style
@whoever # github username
  # Full email
carlo@enfabrica.net
file:sub/OWNERS
`))
	assert.NotNil(t, pb)
	assert.NoError(t, err)

	expected, err := ParseProtoOwners("foo/bar", strings.NewReader(`
action: {
  location: "foo/bar:3"
  review: {
    parent: true
    pattern: ""
    user: {
      location: "foo/bar:3"
      identifier: "@whoever"
    }
    user: {
      location: "foo/bar:5"
      identifier: "carlo@enfabrica.net"
    }
    user: {
      location: "foo/bar:6"
      identifier: "file:sub/OWNERS"
    }
  }
}
`))
	assert.NoError(t, err)
	testutil.AssertProtoEqual(t, pb, expected)

	pb, err = ParseGerritOwners("foo/bar", strings.NewReader(`
# Simple Google style
@whoever # github username
  # Full email
carlo@enfabrica.net
file:sub/OWNERS

# github style code owner
*.bzl carlo@enfabrica.net    @main # example
*.cc				whoever@enfabrica.net   file:/etc/OWNERS @maui#another example

per-file	BUILD=set noparent
per-file	   BUILD     =     @carlo,carlo@enfabrica.net, @luca#ignored

set noparent
*.txt *

include /etc/hosts

per-file	   BUILD     =     @carlo,carlo@enfabrica.net, @mark
`))
	assert.NoError(t, err)
	assert.NotNil(t, pb)

	expected, err = ParseProtoOwners("foo/bar", strings.NewReader(`
action: {
  location: "foo/bar:3"
  review: {
    parent: true
    pattern: ""
    user: {
      location: "foo/bar:3"
      identifier: "@whoever"
    }
    user: {
      location: "foo/bar:5"
      identifier: "carlo@enfabrica.net"
    }
    user: {
      location: "foo/bar:6"
      identifier: "file:sub/OWNERS"
    }
  }
}
action: {
  location: "foo/bar:9"
  review: {
    parent: true
    pattern: "*.bzl"
    user: {
      location: "foo/bar:9"
      identifier: "carlo@enfabrica.net"
    }
    user: {
      location: "foo/bar:9"
      identifier: "@main"
    }
  }
}
action: {
  location: "foo/bar:10"
  review: {
    parent: true
    pattern: "*.cc"
    user: {
      location: "foo/bar:10"
      identifier: "whoever@enfabrica.net"
    }
    user: {
      location: "foo/bar:10"
      identifier: "file:/etc/OWNERS"
    }
    user: {
      location: "foo/bar:10"
      identifier: "@maui"
    }
  }
}
action: {
  location: "foo/bar:12"
  review: {
    pattern: "BUILD"
    user: {
      location: "foo/bar:13"
      identifier: "@carlo"
    }
    user: {
      location: "foo/bar:13"
      identifier: "carlo@enfabrica.net"
    }
    user: {
      location: "foo/bar:13"
      identifier: "@luca"
    }
    user: {
      location: "foo/bar:20"
      identifier: "@mark"
    }
  }
}
action: {
  location: "foo/bar:16"
  review: {
    pattern: "*.txt"
    user: {
      location: "foo/bar:16"
      identifier: "*"
    }
  }
}
action: {
  location: "foo/bar:18"
  include: "/etc/hosts"
}`))
	testutil.AssertProtoEqual(t, pb, expected)
}
