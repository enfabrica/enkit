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

	found, res, err := fc.Root.Find([]string{"user", "non-existing", "blah"})
	assert.Equal(t, fc.User, found, "%v", found)
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, []string{"non-existing", "blah"}, res)
}

type lf struct {
	ns   string
	flag string
}

type lc struct {
	ns      string
	command *kflags.Command
}

type MockAugmenter struct {
	commandCallbacks []func(ns string, command kflags.Command)

	lfs []lf
	lcs []lc
}

func (mr *MockAugmenter) VisitCommand(ns string, command kflags.Command) (bool, error) {
	mr.lcs = append(mr.lcs, lc{ns: ns, command: &command})

	if len(mr.commandCallbacks) > 0 {
		mr.commandCallbacks[0](ns, command)
		mr.commandCallbacks = mr.commandCallbacks[1:]
	}
	return false, nil
}

func (mr *MockAugmenter) VisitFlag(namespace string, flag kflags.Flag) (bool, error) {
	mr.lfs = append(mr.lfs, lf{ns: namespace, flag: flag.Name()})
	return true, nil
}
func (mr *MockAugmenter) Done() error {
	return nil
}

func TestPopulateFlags(t *testing.T) {
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

// This won't build if something is wrong in the KCommand definition.
func TestInterface(t *testing.T) {
	var commander kflags.Commander
	var command kflags.Command

	v := &KCommand{}
	commander = v
	command = v
	t.Log(commander, command)
}

func TestPopulateCommands(t *testing.T) {
	fc := CreateFakeCommand()
	fr := &MockAugmenter{}

	argv := []string{"ignored-argv-0", "user", "add", "justice", "truth"}
	err := PopulateCommands(fc.Root, argv, fr)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(fr.lcs))
	assert.Equal(t, "root.user.add", fr.lcs[0].ns)

	fr = &MockAugmenter{
		commandCallbacks: []func(string, kflags.Command){
			func(ns string, cmd kflags.Command) {
				commander, ok := cmd.(kflags.Commander)
				assert.True(t, ok)
				assert.NotNil(t, commander)

				commander.AddCommand(kflags.CommandDefinition{
					Name:  "justice",
					Short: "never fail to protest",
					Long:  "There may be times when we are powerless to prevent injustice, but there must never be a time when we fail to protest.",
				}, nil, nil)
			},
		},
	}

	err = PopulateCommands(fc.Root, argv, fr)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(fr.lcs))
	assert.Equal(t, "root.user.add", fr.lcs[0].ns)
	assert.Equal(t, "root.user.add.justice", fr.lcs[1].ns)

	// cobra assumes argv[0] is the first sub-command, rather than the path of the binary.
	added, args, err := fc.Root.Find(argv[1:])
	assert.Nil(t, err)
	assert.Equal(t, []string{"truth"}, args)
	assert.Equal(t, "justice", added.Name())
}

func TestAddCommand(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	assert.Equal(t, "Usage:\n", root.UsageString())

	var cbflags []kflags.FlagArg
	var cbargs []string

	kc := KCommand{root}
	kc.AddCommand(kflags.CommandDefinition{
		Name:    "enkit",
		Use:     "-p -f -bah",
		Short:   "shortshortshort",
		Long:    "longlonglong\nlonglonglong",
		Example: "exampleexampleexample\nexampleexampleexample",
		Aliases: []string{"first", "second"},
	}, []kflags.FlagDefinition{
		{
			Name:    "step1",
			Help:    "There can be no peace without justice",
			Default: "peace",
		},
		{
			Name:    "step2",
			Help:    "There can be no justice without truth",
			Default: "justice",
		},
	}, func(flags []kflags.FlagArg, args []string) error {
		cbflags = flags
		cbargs = args
		return nil
	})

	assert.Equal(t, "Usage:\n  root [command]\n\nAvailable Commands:\n  enkit       shortshortshort\n\nUse \"root [command] --help\" for more information about a command.\n", root.UsageString())
	added, args, err := root.Find([]string{"enkit", "dev:stable"})
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, []string{"dev:stable"}, args, err)
	assert.NotNil(t, added)

	assert.Equal(t, "enkit", added.Name())
	assert.Equal(t, "shortshortshort", added.Short)
	expected := `Usage:
  root enkit -p -f -bah [flags]

Aliases:
  enkit, first, second

Examples:
exampleexampleexample
exampleexampleexample

Flags:
      --step1 string   There can be no peace without justice (default "peace")
      --step2 string   There can be no justice without truth (default "justice")
`
	assert.Equal(t, expected, added.UsageString())
	root.SetArgs([]string{"enkit", "--step2", "truth", "fpga:test"})
	err = root.Execute()
	assert.Nil(t, err)

	assert.Equal(t, []string{"fpga:test"}, cbargs)
	assert.Equal(t, 2, len(cbflags))
	assert.Equal(t, "peace", cbflags[0].Value.String())
	assert.Equal(t, "truth", cbflags[1].Value.String())
}
