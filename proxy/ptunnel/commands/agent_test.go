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
	c.SetArgs([]string{"--", "echo", "-n", "$SSH_AUTH_SOCK"})
	b := bytes.NewBufferString("")
	c.SetOut(b)
	assert.Nil(t, c.Execute())
	assert.Equal(t, testAgent.Socket, b.String())
}
