package karchive

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

func TestUnzip(t *testing.T) {
	testCases := []struct {
		desc          string
		path          string
		execErr       error
		wantArgs      []string
		wantDirPrefix string
		wantErr       string
		wantCloseErr  string
	}{
		{
			desc:          "successful unzip",
			path:          "/foo/bar.zip",
			wantArgs:      []string{"unzip", "/foo/bar.zip", "-d"},
			wantDirPrefix: "/tmp/karchive_bar_zip_",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var gotCmd *exec.Cmd
			stubs := gostub.Stub(&runCommand, func(cmd *exec.Cmd) error {
				gotCmd = cmd
				return tc.execErr
			})
			defer stubs.Reset()

			ctx := context.Background()

			got, gotErr := Unzip(ctx, tc.path)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			defer func() {
				errdiff.Check(t, got.Close(), tc.wantCloseErr)
			}()

			for _, arg := range tc.wantArgs {
				assert.Contains(t, gotCmd.Args, arg)
			}
			assert.True(t, strings.HasPrefix(got.tempDir, tc.wantDirPrefix))

		})
	}
}

func TestUnzipActual(t *testing.T) {
	testCases := []struct {
		desc         string
		path         string
		wantFiles    []string
		wantErr      string
		wantCloseErr string
	}{
		{
			desc: "successful unzip",
			path: testutil.MustRunfile("lib/karchive/testdata/small.zip"),
			wantFiles: []string{
				"movies/xmas.txt",
				"tv_shows/cartoons.txt",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			got, gotErr := Unzip(ctx, tc.path)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}
			defer func() {
				errdiff.Check(t, got.Close(), tc.wantCloseErr)
			}()

			for _, f := range tc.wantFiles {
				fullPath := got.Path(f)
				assert.FileExistsf(t, fullPath, "file %q not found in unzipped dir at %q", f, fullPath)
			}
		})
	}
}
