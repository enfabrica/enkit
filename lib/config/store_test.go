package config

import (
	"github.com/enfabrica/enkit/lib/config/directory"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStoreImplementations(t *testing.T) {
	hd, err := directory.OpenHomeDir("application")
	assert.Nil(t, err)

	var _ = []Loader{
		hd,
	}

	var _ = []Store{
		NewSimple(hd, marshal.Json),
		NewMulti(hd, marshal.Toml, marshal.Json),
	}
}
