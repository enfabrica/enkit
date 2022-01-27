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
