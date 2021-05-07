package commands_test

import (
	"bytes"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/kcerts"
	"github.com/enfabrica/enkit/proxy/ptunnel/commands"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRunAgentCommand(t *testing.T) {
	bf := client.DefaultBaseFlags("", "testing")
	testAgent, err := kcerts.FindSSHAgent(bf.Local, bf.Log)
	assert.Nil(t, err)
	c := commands.NewAgentCommand(bf)
	c.SetArgs([]string{"run", "--", "echo", "-n", "$SSH_AUTH_SOCK"})
	b := bytes.NewBufferString("")
	c.SetOut(b)
	assert.Nil(t, c.Execute())
	assert.Equal(t, testAgent.Socket, b.String())
}

func TestRunAgentCommand_Error(t *testing.T) {
	bf := client.DefaultBaseFlags("", "testing")
	c := commands.NewAgentCommand(bf)
	c.SetArgs([]string{"run", "--", "exit", "6"})
	b := bytes.NewBufferString("")
	c.SetOut(b)
	assert.Nil(t, c.Execute())
	assert.Equal(t, "ERROR: exit status 6", b.String())
}
