package commands_test

import (
	"bytes"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/proxy/ptunnel/commands"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os/exec"
	"reflect"
	"testing"
)

func TestRunAgentCommand(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	old := kcerts.GetConfigDir
	defer func() { kcerts.GetConfigDir = old }()
	kcerts.GetConfigDir = func(app string, namespaces ...string) (string, error) {
		return tmpDir + "/.config/enkit", nil
	}

	bf := client.DefaultBaseFlags("", "testing")
	testAgent, err := kcerts.PrepareSSHAgent(bf.Local, kcerts.WithLogging(bf.Log))
	assert.Nil(t, err)
	c := commands.NewAgentCommand(bf)
	c.SetArgs([]string{"run", "--", "echo", "-n", "$SSH_AUTH_SOCK"})
	b := bytes.NewBufferString("")
	c.SetOut(b)
	assert.Nil(t, c.Execute())
	assert.Equal(t, testAgent.Socket, b.String())
}

func TestRunAgentCommand_Error(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "en")
	assert.NoError(t, err)
	old := kcerts.GetConfigDir
	defer func() { kcerts.GetConfigDir = old }()
	kcerts.GetConfigDir = func(app string, namespaces ...string) (string, error) {
		return tmpDir + "/.config/enkit", nil
	}

	bf := client.DefaultBaseFlags("", "testing")
	c := commands.NewAgentCommand(bf)
	c.SetArgs([]string{"run", "--", "exit", "6"})
	b := bytes.NewBufferString("")
	c.SetOut(b)
	assert.Equal(t, reflect.TypeOf(kflags.NewStatusError(6, &exec.ExitError{})), reflect.TypeOf(c.Execute()))
}
