package marshal

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalFile(t *testing.T) {
	data := TestType{
		Name:    "Friedrich",
		Surname: "Nietzsche",
		Year:    1844,
	}
	dir, err := ioutil.TempDir("", "test-marshal")
	assert.NoError(t, err)

	// litmus test: an unknown extension should cause error.
	err = MarshalFile(filepath.Join(dir, "test.whatever"), data)
	assert.Error(t, err)

	err = MarshalFile(filepath.Join(dir, "test.gob"), data)
	assert.NoError(t, err)

	var readback TestType
	err = UnmarshalFile(filepath.Join(dir, "test.gob"), &readback)
	assert.NoError(t, err)

	assert.Equal(t, data, readback)
}

func TestFileMarshallersByExtension(t *testing.T) {
	testCases := []struct {
		desc        string
		path        string
		wantEncoder FileMarshaller
	}{
		{
			desc:        "absolute file path",
			path:        "/foo/bar/baz.json",
			wantEncoder: Json,
		},
		{
			desc:        "relative file path",
			path:        "foo/bar/baz.gob",
			wantEncoder: Gob,
		},
		{
			desc:        "url",
			path:        "https://astore.example.com/g/foo/bar/baz.yaml",
			wantEncoder: Yaml,
		},
		{
			desc:        "url with query parameters",
			path:        "https://astore.example.com/g/foo/bar/baz.toml?u=123abc",
			wantEncoder: Toml,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := FileMarshallers(Known).ByExtension(tc.path)
			assert.Equal(t, got, tc.wantEncoder)
		})
	}
}
