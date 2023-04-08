package kflags

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCamelRewrite(t *testing.T) {
	assert.Equal(t, "", CamelRewrite(""))
	assert.Equal(t, "Foo", CamelRewrite("foo"))
	assert.Equal(t, "FooBar", CamelRewrite("foo-bar"))
	assert.Equal(t, "FooBarBaz", CamelRewrite("foo_bar----baz--"))
}
