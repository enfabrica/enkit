package parser

import (
	"github.com/enfabrica/enkit/ownruler/proto"
	"github.com/go-git/go-billy/osfs"
	"github.com/stretchr/testify/assert"
	"testing"
	"fmt"
)

// CleanUsers removes useless protocol buffer metadata/accessors
// from an array of proto.Users, so it's easy to compare with normal
// assert..* functions.
func CleanUsers(users []proto.User) []proto.User {
	result := []proto.User{}
	for _, u := range users {
		result = append(result, proto.User{
			Identifier: u.Identifier,
			Location: u.Location,
		})
	}
	return result
}

func ExampleAbsPath() {
	fmt.Println(AbsPath("/home/carlo/enkit", "astore", "/lib/test", "file.go"))
	fmt.Println(AbsPath("/home/carlo/enkit", "astore", "lib/test", "file.go"))
	// Output:
        // /home/carlo/enkit/lib/test/file.go
	// /home/carlo/enkit/astore/lib/test/file.go
}

func TestActions(t *testing.T) {
	fs := osfs.New("testdata/fakerepo")
	dopen := FsDirOpener(fs, DefaultIndexes, DefaultLoaders)
	fopen := FsFileOpener(fs, DefaultLoaders)

	revs, nots, err := Actions("test.cc", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "carlo@enfabrica.net", Location: "CODEOWNERS:1"},
		{Identifier: "@ccontavalli", Location: "CODEOWNERS:1"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	revs, nots, err = Actions("BUILD", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@mark", Location: "CODEOWNERS:2"},
		{Identifier: "@marty", Location: "CODEOWNERS:2"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	revs, nots, err = Actions("dir1/foobar", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@pete", Location: "dir1/OWNERS:1"},
		{Identifier: "@mark", Location: "dir1/OWNERS:2"},
		{Identifier: "@george", Location: "dir1/OWNERS:3"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	revs, nots, err = Actions("dir1/BUILD", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@pete", Location: "dir1/OWNERS:1"},
		{Identifier: "@mark", Location: "dir1/OWNERS:2"},
		{Identifier: "@george", Location: "dir1/OWNERS:3"},
		{Identifier: "@marty", Location: "CODEOWNERS:2"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	revs, nots, err = Actions("dir1/myfile.cc", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@pete", Location: "dir1/OWNERS:1"},
		{Identifier: "@mark", Location: "dir1/OWNERS:2"},
		{Identifier: "@george", Location: "dir1/OWNERS:3"},
		{Identifier: "carlo@enfabrica.net", Location: "CODEOWNERS:1"},
		{Identifier: "@ccontavalli", Location: "CODEOWNERS:1"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	// dir2 has a default user, and a 'set noparent'.
	revs, nots, err = Actions("dir2/whatever", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@denny", Location: "dir2/OWNERS:4"},
	}, revs)

	// test.py matches a rule before the 'set noparent'.
	revs, nots, err = Actions("dir2/test.py", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@goo", Location: "dir2/OWNERS:1"},
		{Identifier: "@denny", Location: "dir2/OWNERS:4"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, revs)

	// BUILD matches a rule before the 'set noparent'.
	revs, nots, err = Actions("dir2/BUILD", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@gaa", Location: "dir2/OWNERS:3"},
		{Identifier: "@denny", Location: "dir2/OWNERS:4"},
	}, revs)

	// See if recursion works correctly.
	revs, nots, err = Actions("dir3/dir4/test.cc", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@ccontavalli", Location: "dir3/dir4/METADATA"},
		{Identifier: "@goliath", Location: "dir3/dir4/METADATA"},
		{Identifier: "@tony", Location: "dir3/OWNERS:2"},
		{Identifier: "@henry", Location: "dir3/OWNERS:3"},
		{Identifier: "carlo@enfabrica.net", Location: "CODEOWNERS:1"},
	}, CleanUsers(revs))

	// A .h file should add a notification! Let's see it in action.h
	revs, nots, err = Actions("dir3/dir4/test.h", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, []proto.User{
		{Identifier: "carlo@enfabrica.net", Location: "dir3/dir4/METADATA"},
	}, CleanUsers(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@henry", Location: "dir3/OWNERS:3"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, CleanUsers(revs))

	// Recursion should work even if there is no match in the innermost dir.
	revs, nots, err = Actions("dir3/dir4/BUILD", dopen, fopen)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(nots))
	assert.Equal(t, []proto.User{
		{Identifier: "@dude", Location: "dir3/OWNERS:1"},
		{Identifier: "@henry", Location: "dir3/OWNERS:3"},
		{Identifier: "@mark", Location: "CODEOWNERS:2"},
		{Identifier: "@marty", Location: "CODEOWNERS:2"},
		{Identifier: "@tony", Location: "CODEOWNERS:3"},
	}, CleanUsers(revs))

	// Let's try some simple includes.
	//
	// TODO(carlo): this test does not pass! Suspected problem:
	//   the patterns in an included file are considered relative
	//   to the path of the file with the include statement, rather
	//   than the path of the file defining the patterns. Need an
	//   extra parameter in the matcher.
	//
	// revs, nots, err = Actions("dir4/BUILD", dopen, fopen)
	// assert.NoError(t, err)
	// assert.Equal(t, 0, len(nots))
	// assert.Equal(t, []proto.User{
	// 	{Identifier: "@gastone", Location: "dir4/OWNERS:1"},
	// 	{Identifier: "@tom", Location: "dir4/mygroup.codeowners:1"},
	// 	{Identifier: "@jerry", Location: "dir4/mygroup.codeowners:4"},
	// 	{Identifier: "@qui", Location: "dir4/OWNERS:3"},
	// }, CleanUsers(revs))
}
