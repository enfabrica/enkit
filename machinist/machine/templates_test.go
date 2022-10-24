package machine_test

import (
	"fmt"
	"github.com/enfabrica/enkit/machinist/machine"
	"github.com/enfabrica/enkit/machinist/machine/assets"
	"github.com/stretchr/testify/assert"

	"testing"
)
// Todo(adam): validate tempalte with nss somehow calling the parse lib
func TestMachinistNodeTemplate(t *testing.T) {
	_, err := machine.ReadSSHDContent("/bar", "/foo", "/baz")
	assert.Nil(t, err)
	for k := range assets.AutoUserBinaries {
		fmt.Println(k)
	}
	c := &machine.NssConf{
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
	out, err := machine.ReadNssConf(c)
	assert.Nil(t, err)
	fmt.Print(string(out))
}
