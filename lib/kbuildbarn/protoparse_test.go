package kbuildbarn_test

import (
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEmpty(t *testing.T) {
	result := kbuildbarn.GenerateLinksForFiles([]*bespb.File{}, "", "", "")
	assert.Nil(t, result)
}

func TestSingleContain(t *testing.T) {
	simple := []*bespb.File{
		{
			Name: "simple.txt", Digest: "digest", Length: 614,
		},
	}
	result := kbuildbarn.GenerateLinksForFiles(simple, "/enfabrica/mymount", "myInvocation", "localCluster")
	assert.Equal(t, "/enfabrica/mymount/cas/localCluster/blobs/file/digest", result[0].Src)
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
	expected := map[string]string{
		"/foo/bar/scratch/invocation/simple.txt":       "/foo/bar/cas/duster/blobs/file/digest0",
		"/foo/bar/scratch/invocation/hello/simple.txt": "/foo/bar/cas/duster/blobs/file/digest1",
		"/foo/bar/scratch/invocation/one/two/foo.bar":  "/foo/bar/cas/duster/blobs/file/digest2",
		"/foo/bar/scratch/invocation/tarball.tar":      "/foo/bar/cas/duster/blobs/file/digest3",
	}
	r := kbuildbarn.GenerateLinksForFiles(many, baseName, invocationPrefix, clusterName)
	for expectedDest, expectedSim := range expected {
		foundByDest := r.FindByDest(expectedDest)
		foundBySim := r.FindBySrc(expectedSim)
		assert.NotNil(t, foundBySim)
		assert.NotNil(t, foundByDest)
		assert.Equal(t, foundBySim, foundByDest)
		assert.Equal(t, expectedDest, foundByDest.Dest)
		assert.Equal(t, expectedSim, foundBySim.Src)
	}
}
