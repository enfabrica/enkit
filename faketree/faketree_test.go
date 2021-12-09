package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDefaultShell(t *testing.T) {
	assert.Equal(t, "/bin/sh", DefaultShell())
	os.Setenv("SHELL", "/bin/false")
	assert.Equal(t, "/bin/false", DefaultShell())
}

func TestLookup(t *testing.T) {
	uid, gid, err := ParseOrLookupUser("-12")
	assert.Error(t, err)
	assert.Zero(t, uid)
	assert.Zero(t, gid)

	u, err := user.Current()
	assert.NoError(t, err)

	uid, gid, err = ParseOrLookupUser(u.Username)
	assert.NoError(t, err)
	assert.Equal(t, u.Uid, strconv.Itoa(uid))
	assert.Equal(t, u.Gid, strconv.Itoa(gid))

	uid, gid, err = ParseOrLookupUser(u.Uid)
	assert.NoError(t, err)
	assert.Equal(t, u.Uid, strconv.Itoa(uid))
	assert.Equal(t, 0, gid)

	gid, err = ParseOrLookupGroup("")
	assert.Error(t, err)
	assert.Equal(t, 0, gid)

	gid, err = ParseOrLookupGroup(u.Gid)
	assert.NoError(t, err)
	assert.Equal(t, u.Gid, strconv.Itoa(gid))
}

func TestParseFlags(t *testing.T) {
	fl := NewFlags()

	u, err := user.Current()
	assert.NoError(t, err)

	assert.NotZero(t, fl.Perms)
	assert.True(t, filepath.IsAbs(fl.Faketree))

	assert.Equal(t, u.Uid, strconv.Itoa(fl.Uid))
	assert.Equal(t, u.Gid, strconv.Itoa(fl.Gid))

	left, err := fl.Parse([]string{
		"--marx-was-right",
	})
	assert.Error(t, err)
	assert.Equal(t, 0, len(left))

	left, err = fl.Parse([]string{
		"--uid=12", "--", "I am fond of pigs",
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"I am fond of pigs"}, left)
	assert.Equal(t, 12, fl.Uid)
	assert.Equal(t, u.Gid, strconv.Itoa(fl.Gid))

	args := fl.Args()
	assert.Equal(t, []string{"--uid", "12", "--gid", u.Gid, "--faketree", fl.Faketree}, args)

	left, err = fl.Parse([]string{
		"--uid=12", "--mount", "/etc/hosts:/tmp/hosts", "--mount", "/proc:/tmp/proc", "--", "no pig",
	})

	args = fl.Args()
	assert.Equal(t, []string{"--uid", "12", "--gid", u.Gid, "--faketree", fl.Faketree, "--mount", "/etc/hosts:/tmp/hosts", "--mount", "/proc:/tmp/proc"}, args)
}
