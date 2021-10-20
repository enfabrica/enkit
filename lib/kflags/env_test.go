package kflags

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCamelRewrite(t *testing.T) {
	assert.Equal(t, "", CamelRewrite(""))
	assert.Equal(t, "Foo", CamelRewrite("foo"))
	assert.Equal(t, "FooBar", CamelRewrite("foo-bar"))
	assert.Equal(t, "FooBarBaz", CamelRewrite("foo_bar----baz--"))
}
