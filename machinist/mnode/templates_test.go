package mnode_test

import (
	"fmt"
	"github.com/enfabrica/enkit/machinist/machinist_assets"
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/stretchr/testify/assert"

	"testing"
)
// Todo(adam): validate tempalte with nss somehow calling the parse lib
func TestMachinistNodeTemplate(t *testing.T) {
	_, err := mnode.ReadSSHDContent("/bar", "/foo", "/baz")
	assert.Nil(t, err)
	for k := range machinist_assets.Data {
		fmt.Println(k)
	}
	c := &mnode.NssConf{
		DefaultShell: "/home/shelly",
		Shells: []struct {
			Home  string
			Shell string
			Match string
		}{
			{
				Home:  "/home/meow",
				Match: "hello",
			},
			{
				Home:  "/home/uwu",
				Shell: "/bin/tty",
				Match: "ocarina",
			},
		},
	}
	out, err := mnode.ReadNssConf(c)
	assert.Nil(t, err)
	fmt.Print(string(out))
}
