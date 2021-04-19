package mnode_test

import (
	"fmt"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestMachinistNodeTemplate(t *testing.T) {
	r, err  :=  mnode.ReadSSHDContent("/bar", "/foo", "/baz")
	assert.Nil(t, err)
}
