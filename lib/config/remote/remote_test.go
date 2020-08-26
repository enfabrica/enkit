package remote

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	o, err := ParseDNSOptions("")
	assert.Nil(t, err)
	assert.Nil(t, o)

	o, err = ParseDNSOptions("foo=bar")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions{"foo": "bar"}, o)

	o, err = ParseDNSOptions("foo%20=bar   bar=baz")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions{"foo": "bar", "bar": "baz"}, o)

	o, err = ParseDNSOptions("foo%20=bar   bar=baz =")
	assert.NotNil(t, err)

	o, err = ParseDNSOptions("foo%=bar   bar=baz")
	assert.NotNil(t, err)

	o, err = ParseDNSOptions("foo=bar   bar")
	assert.NotNil(t, err)
}

func TestParseTXT(t *testing.T) {
	o, u, err := ParseTXTRecord("foo=bar|https://www.google.com")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions{"foo": "bar"}, o)
	assert.Equal(t, "https://www.google.com", u.String())

	o, u, err = ParseTXTRecord("|https://www.google.com")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions(nil), o)
	assert.Equal(t, "https://www.google.com", u.String())

	o, u, err = ParseTXTRecord("   |https://www.google.com")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions(nil), o)
	assert.Equal(t, "https://www.google.com", u.String())

	o, u, err = ParseTXTRecord("https://www.google.com")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions(nil), o)
	assert.Equal(t, "https://www.google.com", u.String())

	o, u, err = ParseTXTRecord("www.google.com")
	assert.Nil(t, err)
	assert.Equal(t, DNSOptions(nil), o)
	assert.Equal(t, "www.google.com", u.String())
}

func TestDNSOptionsApply(t *testing.T) {
	o, err := ParseDNSOptions("timeout=3s retries=12")
	assert.Nil(t, err)

	unknown, err := o.Apply(nil)
	assert.NotNil(t, err)

	dest1 := &struct{}{}
	unknown, err = o.Apply(&dest1)
	assert.Equal(t, []string{"timeout", "retries"}, unknown)

	dest2 := &struct {
		Timeout time.Duration
	}{}
	unknown, err = o.Apply(&dest2)
	assert.Equal(t, []string{"retries"}, unknown)
	assert.Equal(t, 3*time.Second, dest2.Timeout)

	dest3 := &struct {
		Timeout time.Duration
		Retries int
	}{}
	unknown, err = o.Apply(&dest3)
	assert.Equal(t, []string{}, unknown)
	assert.Equal(t, 3*time.Second, dest3.Timeout)
	assert.Equal(t, 12, dest3.Retries)

	o, err = ParseDNSOptions("timeout=invalid retries=12")
	unknown, err = o.Apply(&dest3)
	assert.NotNil(t, err, "%s", err)
}
