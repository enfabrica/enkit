package kcobra

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"testing"
)

type FakeCommand struct {
	Root, Artifact, Upload, Download, User, Add, System, Del, Login *cobra.Command

	RootF string
	RootP bool

	ArtifactF []string
	ArtifactP int

	UploadF string
	UploadP string
	SystemF string
	SystemP string
	AddF    string
	AddP    string
}

func CreateFakeCommand() *FakeCommand {
	root := &cobra.Command{Use: "root"}

	artifact := &cobra.Command{Use: "artifact"}
	upload := &cobra.Command{Use: "upload"}
	download := &cobra.Command{Use: "download"}

	user := &cobra.Command{Use: "user"}
	add := &cobra.Command{Use: "add"}
	system := &cobra.Command{Use: "system"}

	del := &cobra.Command{Use: "del"}

	login := &cobra.Command{Use: "login"}

	artifact.AddCommand(upload)
	artifact.AddCommand(download)

	add.AddCommand(system)
	user.AddCommand(add)
	user.AddCommand(del)

	root.AddCommand(artifact)
	root.AddCommand(user)
	root.AddCommand(login)

	ff := &FakeCommand{
		Root: root, Artifact: artifact, Upload: upload, Download: download, User: user, Add: add, System: system, Del: del, Login: login,
	}

	root.Flags().StringVarP(&ff.RootF, "root-f", "a", "rootf0", "rootf0")
	root.PersistentFlags().BoolVarP(&ff.RootP, "root-p", "b", true, "rootp0")
	artifact.Flags().StringArrayVarP(&ff.ArtifactF, "artifact-f", "c", []string{"artifactf0", "artifactf1"}, "artifactf0")
	artifact.PersistentFlags().IntVarP(&ff.ArtifactP, "artifact-p", "d", 42, "artifactp0")
	upload.Flags().StringVarP(&ff.UploadF, "upload-f", "e", "uploadf0", "uploadf0")
	upload.PersistentFlags().StringVarP(&ff.UploadP, "upload-p", "f", "uploadp0", "uploadp0")
	system.Flags().StringVarP(&ff.SystemF, "system-f", "g", "systemf0", "systemf0")
	system.PersistentFlags().StringVarP(&ff.SystemP, "system-p", "i", "systemp0", "systemp0")
	add.Flags().StringVarP(&ff.AddF, "add-f", "j", "addf0", "addf0")
	add.PersistentFlags().StringVarP(&ff.AddP, "add-p", "k", "addp0", "addp0")

	return ff
}

var rootUsage = `Usage:

Flags:
  -a, --root-f string   rootf0 (default "rootf0")
  -b, --root-p          rootp0 (default true)

Additional help topics:
  root artifact 
  root login    
  root user     
`

var addUsage = `Usage:

Flags:
  -j, --add-f string   addf0 (default "addf0")
  -k, --add-p string   addp0 (default "addp0")

Global Flags:
  -b, --root-p   rootp0 (default true)

Additional help topics:
  root user add system 
`

var systemUsage = `Usage:

Flags:
  -g, --system-f string   systemf0 (default "systemf0")
  -i, --system-p string   systemp0 (default "systemp0")

Global Flags:
  -k, --add-p string   addp0 (default "addp0")
  -b, --root-p         rootp0 (default true)
`

var systemUsageChanged = `Usage:

Flags:
  -g, --system-f string   systemf0 (default "changed1")
  -i, --system-p string   systemp0 (default "systemp0")

Global Flags:
  -k, --add-p string   addp0 (default "addp0")
  -b, --root-p         rootp0
`

// This package relies on a few behaviors of the cobra and flags library.
// This test verifies that the behavior of cobra has not changed.
// It is not expected to fail unless you run it against a newer version of cobra.
func TestCobraFeatures(t *testing.T) {
	fc := CreateFakeCommand()
	assert.Equal(t, rootUsage, fc.Root.UsageString())
	assert.Equal(t, addUsage, fc.Add.UsageString())
	assert.Equal(t, systemUsage, fc.System.UsageString())

	f := fc.System.Flags().Lookup("system-f")
	f.Value.Set("changed1")
	f.DefValue = "changed1"
	assert.NotNil(t, f)
	f = fc.Root.Flags().Lookup("root-p")
	f.Value.Set("false")
	f.DefValue = "false"
	assert.NotNil(t, f)

	assert.Equal(t, systemUsageChanged, fc.System.UsageString())

	cmd, g, err := fc.Root.Find([]string{})
	assert.Nil(t, err)
	assert.Equal(t, fc.Root, cmd)
	assert.Equal(t, []string{}, g)

	cmd, g, err = fc.Root.Find([]string{"user", "-j", "foo", "-g", "bar", "add", "system", "arg1", "arg2", "-h", "another"})
	assert.Nil(t, err)
	assert.Equal(t, fc.System, cmd)
	assert.Equal(t, []string{"-j", "foo", "-g", "bar", "arg1", "arg2", "-h", "another"}, g)

	count := 0
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		count += 1
	})
	assert.Equal(t, 4, count)

	fc.System.RunE = func(cmd *cobra.Command, args []string) error {
		assert.False(t, cmd.Flags().Changed("system-f"))
		assert.False(t, cmd.Flags().Changed("root-p"))
		assert.Equal(t, "changed1", fc.SystemF)
		assert.Equal(t, false, fc.RootP)
		return nil
	}

	fc.Root.SetArgs([]string{"user", "add", "system"})
	err = fc.Root.Execute()
	assert.Nil(t, err)

	cobra.MarkFlagRequired(fc.Root.PersistentFlags(), "root-p")
	fc.Root.SetArgs([]string{"user", "add", "system"})
	err = fc.Root.Execute()
	assert.NotNil(t, err)
}

type lf struct {
	ns   string
	flag string
}

type MockAugmenter struct {
	lfs []lf
}

func (mr *MockAugmenter) Visit(namespace string, flag kflags.Flag) (bool, error) {
	mr.lfs = append(mr.lfs, lf{ns: namespace, flag: flag.Name()})
	return true, nil
}
func (mr *MockAugmenter) Done() error {
	return nil
}

func TestPopulateDefaults(t *testing.T) {
	fc := CreateFakeCommand()
	fr := &MockAugmenter{}

	err := PopulateDefaults(fc.Root, []string{"ignored-argv-0", "user", "add", "system"}, fr)
	assert.Nil(t, err)

	// user has no direct flags, only subcommands.
	// Still, it should show up with the persistent flag of the parent.
	assert.Equal(t, []lf{
		{ns: "root", flag: "root-f"},
		{ns: "root", flag: "root-p"},

		{ns: "root.user", flag: "root-p"},

		{ns: "root.user.add", flag: "root-p"},
		{ns: "root.user.add", flag: "add-f"},
		{ns: "root.user.add", flag: "add-p"},

		{ns: "root.user.add.system", flag: "add-p"},
		{ns: "root.user.add.system", flag: "root-p"},
		{ns: "root.user.add.system", flag: "system-f"},
		{ns: "root.user.add.system", flag: "system-p"},
	}, fr.lfs)
}
