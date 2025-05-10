package kassets

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

//go:embed testdata/*
var testdataFS embed.FS

func TestNewAssetAugmenter(t *testing.T) {
	subdir, err := EmbedSubdir(testdataFS, "testdata")
	require.NoError(t, err)
	assetMap, err := MapFromFS(subdir)
	require.NoError(t, err)

	assert.ElementsMatch(
		t,
		maps.Keys(assetMap),
		[]string{
			"heroes",
			"heroes.txt",
			"villains",
			"villains.extra",
			"villains.extra.ext",
			"villains.extra.ext.txt",
		},
	)
}
