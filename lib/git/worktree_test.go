package git

import (
	"testing"
	"os/exec"

	"github.com/enfabrica/enkit/lib/errdiff"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

func TestNewTempWorktree(t *testing.T) {
	testCases := []struct {
		desc string
		committish string
		wantCmds []*exec.Cmd
		wantErr string
		wantCloseErr string
	} {
		{
			desc: "successfully creates temp worktree",
			committish: "some_branch_name",
			wantCmds: []*exec.Cmd{
				&exec.Cmd{
					Path: "git",
					Args: []string{"worktree", "add", "foo", "some_branch_name"},
					Dir: "/foo/bar",
				},
				&exec.Cmd{
					Path: "git",
					Args: []string{"worktree", "remove", "foo"},
					Dir: "/foo/bar",
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotCmds := []*exec.Cmd{}
			stubs := gostub.Stub(&runCommand, func(cmd *exec.Cmd) ([]byte, error) {
				gotCmds = append(gotCmds, cmd)
				return nil, nil
			})
			defer stubs.Reset()

			got, gotErr := NewTempWorktree("/foo/bar", tc.committish)
			closeErr := got.Close()

			if diff := errdiff.Substring(gotErr, tc.wantErr); diff != "" {
				t.Error(diff)
			}
			if diff := errdiff.Substring(closeErr, tc.wantCloseErr); diff != "" {
				t.Error(diff)
			}
			assert.Equal(t, 2, len(gotCmds))
			assert.Equal(t, []string{"git", "worktree", "add"}, gotCmds[0].Args[0:3])
			assert.Equal(t, []string{"some_branch_name"}, gotCmds[0].Args[4:])
			assert.Equal(t, "/foo/bar", gotCmds[0].Dir)
			assert.Equal(t, []string{"git", "worktree", "remove"}, gotCmds[1].Args[0:3])
			assert.Equal(t, "/foo/bar", gotCmds[1].Dir)
		})
	}
}