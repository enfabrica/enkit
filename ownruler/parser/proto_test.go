package parser

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestParseProtoOwners(t *testing.T) {
	// Empty config is a valid config.
	ow, err := ParseProtoOwners("foo/bar", strings.NewReader(""))
	assert.NotNil(t, ow)
	assert.NoError(t, err)

	// Truly invalid config.
	ow, err = ParseProtoOwners("foo/bar", strings.NewReader("foo"))
	assert.Nil(t, ow)
	assert.Error(t, err)

	// Reasonable config.
	ow, err = ParseProtoOwners("foo/bar", strings.NewReader(`
location: "whatever/path"
action: {
  location: "foo"
  review: {
    pattern: ".*"
    user:  {
      identifier: "@foo"
    }
    user: {
      identifier: "@bar"
    }
  }
}

action: {
  notify: {
    pattern: ".*"
    user: {
      identifier: "@carlo"
    }
  }
}`))
	assert.NotNil(t, ow)
	assert.Nil(t, err)

	// ParseProtoOwners is also used in the gerrit_test, to compare
	// the parsed OWNERS/CODEOWNERS files with protocol buffers.
}
