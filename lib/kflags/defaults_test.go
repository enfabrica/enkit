package kflags

import (
	"flag"
	"github.com/stretchr/testify/assert"
	"testing"
)

// Verifies flag registration and help screen.
func TestArrayFlagsDefault(t *testing.T) {
	fs := flag.NewFlagSet("fake-test-flags", flag.ContinueOnError)
	gfs := &GoFlagSet{fs}

	var array []string
	gfs.StringArrayVar(&array, "array", []string{"one", "two"}, "test value")

	assert.Nil(t, fs.Parse([]string{}))
	assert.Equal(t, []string{"one", "two"}, array)
}

func TestArrayFlagsValues(t *testing.T) {
	fs := flag.NewFlagSet("fake-test-flags", flag.ContinueOnError)
	gfs := &GoFlagSet{fs}

	var array []string
	gfs.StringArrayVar(&array, "array", []string{"one", "two"}, "test value")

	assert.Nil(t, fs.Parse([]string{"-array", "test1", "-array", "test2", "-array=test3", "--array=test4"}))
	assert.Equal(t, []string{"test1", "test2", "test3", "test4"}, array)
}

func TestArrayFlagsNilDefault(t *testing.T) {
	fs := flag.NewFlagSet("fake-test-flags", flag.ContinueOnError)
	gfs := &GoFlagSet{fs}

	var array []string
	gfs.StringArrayVar(&array, "array", nil, "test value")

	assert.Nil(t, fs.Parse([]string{"-array", "test1", "-array", "test2", "-array=test3", "--array=test4"}))
	assert.Equal(t, []string{"test1", "test2", "test3", "test4"}, array)
}
