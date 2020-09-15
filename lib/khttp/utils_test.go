package khttp

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func ExampleSplitHostPort() {
	fmt.Println(SplitHostPort("1.2.3.4:53"))
	fmt.Println(SplitHostPort("1.2.3.4"))
	fmt.Println(SplitHostPort(":53"))
	fmt.Println(SplitHostPort(":"))

	// Taken to be a host, no port.
	fmt.Println(SplitHostPort("53"))

	// Strings are supported, as they can be resolved into
	// numbers using net.LookupPort and similar.
	fmt.Println(SplitHostPort("server:ssh"))

	// IPv6 is well supported.
	fmt.Println(SplitHostPort("[::1]:12"))

	// Output:
	// 1.2.3.4 53 <nil>
	// 1.2.3.4  <nil>
	//  53 <nil>
	//   <nil>
	// 53  <nil>
	// server ssh <nil>
	// ::1 12 <nil>
}

func TestSplitHostPort(t *testing.T) {
	host, port, err := SplitHostPort("")
	assert.Equal(t, "", host)
	assert.Equal(t, "", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("53")
	assert.Equal(t, "53", host)
	assert.Equal(t, "", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort(":53")
	assert.Equal(t, "", host)
	assert.Equal(t, "53", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("fuffa:53")
	assert.Equal(t, "fuffa", host)
	assert.Equal(t, "53", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("foo:bar")
	assert.Equal(t, "foo", host)
	assert.Equal(t, "bar", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("[::1]:53")
	assert.Equal(t, "::1", host)
	assert.Equal(t, "53", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("[::1]:")
	assert.Equal(t, "::1", host)
	assert.Equal(t, "", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("[::1]")
	assert.Equal(t, "::1", host)
	assert.Equal(t, "", port)
	assert.Nil(t, err)

	host, port, err = SplitHostPort("[::1]::53")
	assert.NotNil(t, err)
}
