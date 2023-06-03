package khttp

import (
	"github.com/stretchr/testify/assert"
	"github.com/enfabrica/enkit/lib/kflags"
	"os"
	"flag"
	"testing"
)

func TestEnvPort(t *testing.T) {
	// Ensure the test is not picking up a PORT parameter from our CI/CD.
	os.Unsetenv("PORT")

	// No port, simple default.
	assert.Equal(t, 12345, EnvPort(12345))
	// Manually set default is returned as is even if not a valid port.
	assert.Equal(t, 1234567, EnvPort(1234567))

	// Empty port number is ignored.
	assert.NoError(t, os.Setenv("PORT", ""))
	assert.Equal(t, 12345, EnvPort(12345))

	// Invalid port number is ignored.
	assert.NoError(t, os.Setenv("PORT", "not-a-number"))
	assert.Equal(t, 12345, EnvPort(12345))
	assert.NoError(t, os.Setenv("PORT", "123456789"))
	assert.Equal(t, 12345, EnvPort(12345))
	assert.NoError(t, os.Setenv("PORT", "0"))
	assert.Equal(t, 12345, EnvPort(12345))
	assert.NoError(t, os.Setenv("PORT", "-1"))
	assert.Equal(t, 12345, EnvPort(12345))

	// Valid port number is used.
	assert.NoError(t, os.Setenv("PORT", "5421"))
	assert.Equal(t, 5421, EnvPort(12345))
}

func TestAddDefaultPort(t *testing.T) {
	address, err := addDefaultPort("", 0)
	assert.Error(t, err)

	address, err = addDefaultPort("", 65536)
	assert.Error(t, err)

	address, err = addDefaultPort("", 80)
	assert.NoError(t, err)
	assert.Equal(t, ":80", address)

	address, err = addDefaultPort("127.0.0.1", 80)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:80", address)

	address, err = addDefaultPort("[::1]", 80)
	assert.NoError(t, err)
	assert.Equal(t, "[::1]:80", address)

	address, err = addDefaultPort("127.0.0.1:1234", 80)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:1234", address)

	address, err = addDefaultPort("[::1]:1234", 80)
	assert.NoError(t, err)
	assert.Equal(t, "[::1]:1234", address)
}

func GetAllFlags(fs *flag.FlagSet) []string {
	flags := []string{}
	fs.VisitAll(func (fl *flag.Flag) {
		flags = append(flags, fl.Name)
	})
	return flags
}

func TestFlags(t *testing.T) {
	flags := DefaultFlags()

	// Make sure register panics if some flags are duplicate.
	set := &kflags.GoFlagSet{FlagSet: flag.NewFlagSet("test", flag.PanicOnError)}
	flags.Register(set, "test-prefix-")

	// Verify all registered flags have been prefixed correctly.
	found := GetAllFlags(set.FlagSet)
	for _, fl := range found {
		assert.Regexp(t, `^test-prefix-[^-]`, fl)
	}
}
