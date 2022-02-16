package kbuildbarn

import (
	"testing"

	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"

	"github.com/stretchr/testify/assert"
)

func TestEmpty(t *testing.T) {
	result := GenerateLinksForFiles([]*bespb.File{}, "", "", "", "")
	assert.Nil(t, result)
}

func TestSingleContain(t *testing.T) {
	simple := []*bespb.File{
		{
			Name: "simple.txt", Digest: "digest", Length: 614,
		},
	}
	result := GenerateLinksForFiles(simple, "/enfabrica/mymount", "", "myInvocation", "localCluster")
	assert.Equal(t, "/enfabrica/mymount/cas/localCluster/blobs/file/digest-614", result[0].Src)
	assert.Equal(t, "/enfabrica/mymount/scratch/myInvocation/simple.txt", result[0].Dest)
}

func TestParseMany(t *testing.T) {
	many := []*bespb.File{
		{
			Name: "simple.txt", Digest: "digest0", Length: 614,
		},
		{
			Name: "hello/simple.txt", Digest: "digest1", Length: 43,
		},
		{
			Name: "one/two/foo.bar", Digest: "digest2", Length: 888,
		},
		{
			Name: "tarball.tar", Digest: "digest3", Length: 777,
		},
	}
	baseName := "/foo/bar"
	clusterName := "duster"
	invocationPrefix := "invocation"
	expected := HardlinkList{
		&Hardlink{
			Src:  "/foo/bar/cas/duster/blobs/file/digest0-614",
			Dest: "/foo/bar/scratch/invocation/subdir/simple.txt",
		},
		&Hardlink{
			Src:  "/foo/bar/cas/duster/blobs/file/digest1-43",
			Dest: "/foo/bar/scratch/invocation/subdir/hello/simple.txt",
		},
		&Hardlink{
			Src:  "/foo/bar/cas/duster/blobs/file/digest2-888",
			Dest: "/foo/bar/scratch/invocation/subdir/one/two/foo.bar",
		},
		&Hardlink{
			Src:  "/foo/bar/cas/duster/blobs/file/digest3-777",
			Dest: "/foo/bar/scratch/invocation/subdir/tarball.tar",
		},
	}
	r := GenerateLinksForFiles(many, baseName, "subdir", invocationPrefix, clusterName)
	assert.ElementsMatch(t, r, expected)
}
