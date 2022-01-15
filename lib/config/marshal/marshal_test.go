package marshal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestType struct {
	Name, Surname string
	Year          int
}

func TestGobMarshal(t *testing.T) {
	data := TestType{
		Name:    "Friedrich",
		Surname: "Nietzsche",
		Year:    1844,
	}

	result, err := Gob.Marshal(data)
	assert.NoError(t, err)
	assert.True(t, len(result) > 0)

	var comparison TestType
	err = Gob.Unmarshal(result, &comparison)
	assert.NoError(t, err)

	assert.Equal(t, data, comparison)
}
