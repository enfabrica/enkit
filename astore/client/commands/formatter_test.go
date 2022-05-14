package commands

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/stretchr/testify/assert"
)

func TestMarshalFormat(t *testing.T) {
	artifact := astore.Artifact {
		Created: 1234,
		Creator: "Friedrich Nietzsche",
		Architecture: "amd64",
		MD5: []byte{102, 97, 108, 99, 111, 110},
		Uid: "uid-string",
		Sid: "sid-string",
		Tag: []string{"tag1", "tag2"},
		Note: "note1",
	}

	element := astore.Element {
		Created: 1234,
		Creator: "Friedrich Nietzsche",
		Name: "foo-element",
	}

	dir, err := ioutil.TempDir("", "test-marshal")
	assert.NoError(t, err)

	for _, ext := range []string{ "json", "yaml" } {
		root := NewRoot(nil)
		root.outputFile = filepath.Join(dir, "test." + ext)

		formatter := root.Formatter()
		formatter.Artifact(&artifact)
		formatter.Element(&element)
		formatter.Flush()

		// check that the destination was created
		_, err = os.Open(root.outputFile)
		assert.NoError(t, err)
	}

}
