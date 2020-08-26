package kflags

import (
	"bytes"
	"flag"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// Verifies flag registration and help screen.
func TestBinaryFileRendering(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	dest := []byte{}
	fs.Var(NewByteFileFlag(&dest, ""), "file", "loads a file")
	err := fs.Parse([]string{})
	assert.Nil(t, err)
	assert.Equal(t, []byte{}, dest)

	buffer := bytes.Buffer{}
	fs.SetOutput(&buffer)
	fs.PrintDefaults()

	assert.Equal(t, "  -file value\n    \tloads a file\n", buffer.String())
	buffer.Reset()

	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(&buffer)
	fs.Var(NewByteFileFlag(&dest, "/tmp/test"), "file", "loads a file")
	fs.PrintDefaults()

	assert.Equal(t, "  -file value\n    \tloads a file (default /tmp/test)\n", buffer.String())
	buffer.Reset()
}

// Test that actually causes the flag to attempt to read a file.
func TestBinaryFileRead(t *testing.T) {
	quote := "An ounce of action is worth a ton of theory."
	d, err := ioutil.TempDir(".", "tempdir-*.data")
	assert.Nil(t, err)
	fpath := filepath.Join(d, "testfile")
	err = ioutil.WriteFile(fpath, []byte(quote), 0600)
	assert.Nil(t, err)

	dest := []byte{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(NewByteFileFlag(&dest, "/tmp/test"), "file", "loads a file")
	err = fs.Parse([]string{"-file", "/does/not/exist/for/sure00000"})
	assert.NotNil(t, err)

	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(NewByteFileFlag(&dest, "/tmp/test"), "file", "loads a file")
	err = fs.Parse([]string{"-file", fpath})
	assert.Nil(t, err)
	assert.Equal(t, []byte(quote), dest)
}

func TestBinaryFileDefault(t *testing.T) {
	quote := "Those who do not move, do not notice their chains."
	d, err := ioutil.TempDir(".", "tempdir-*.data")
	assert.Nil(t, err)
	fpath := filepath.Join(d, "testfile")
	err = ioutil.WriteFile(fpath, []byte(quote), 0600)
	assert.Nil(t, err)

	dest := []byte{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(NewByteFileFlag(&dest, fpath), "file", "loads a file")
	err = fs.Parse([]string{})
	assert.Nil(t, err)
	assert.Equal(t, []byte(quote), dest)
}
