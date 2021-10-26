package git

import (
	"os/exec"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

func TestNewTempWorktree(t *testing.T) {
	gotCmds := []*exec.Cmd{}
	stubs := gostub.Stub(&runCommand, func(cmd *exec.Cmd) ([]byte, error) {
		gotCmds = append(gotCmds, cmd)
		return nil, nil
	})
	defer stubs.Reset()

	got, gotErr := NewTempWorktree("/foo/bar", "some_branch_name")
	closeErr := got.Close()

	assert.NoError(t, gotErr)
	assert.NoError(t, closeErr)
	assert.Equal(t, 2, len(gotCmds))
	assert.Equal(t, []string{"git", "worktree", "add", "--detach"}, gotCmds[0].Args[0:4])
	assert.Equal(t, []string{"some_branch_name"}, gotCmds[0].Args[5:])
	assert.Equal(t, "/foo/bar", gotCmds[0].Dir)
	assert.Equal(t, []string{"git", "worktree", "remove"}, gotCmds[1].Args[0:3])
	assert.Equal(t, "/foo/bar", gotCmds[1].Dir)
}
