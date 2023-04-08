package kflags

import (
	"flag"
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"path/filepath"
	"strings"
)

// Flag represents a command line flag.
type Flag interface {
	// Returns the name of the flag.
	Name() string
	// Sets the value of the flag.
	Set(string) error
	// Sets the content of the flag (for those flags that support it,
	// see the description of the ContentValue interface for more details)
	SetContent(string, []byte) error
}

// GoFlagSet wraps a flag.FlagSet from the go standard library and completes the
// implementation of the FlagSet interface in this module.
//
// For example, to use the default "flag" library FlagSet:
//
//     var set kflags.FlagSet
//     set = &kflags.GoFlagSet{FlagSet: flag.CommandLine}
//
type GoFlagSet struct {
	*flag.FlagSet
}

func (fs *GoFlagSet) ByteFileVar(p *[]byte, name string, defaultFile string, usage string, mods ...ByteFileModifier) {
	fs.Var(NewByteFileFlag(p, defaultFile, mods...), name, usage)
}

// stringArrayFlag is a simple implementation of the flag.Value interface to provide array of flags.
type stringArrayFlag struct {
	dest     *[]string
	defaults bool // set to true if dest points to the default value.
}

func (v *stringArrayFlag) String() string {
	return fmt.Sprintf("%v", *v.dest)
}

func (v *stringArrayFlag) Set(value string) error {
	// Don't modify/append to the default array, create a new one.
	if v.defaults {
		*v.dest = []string{}
		v.defaults = false
	}

	*v.dest = append(*v.dest, value)
	return nil
}

func (fs *GoFlagSet) StringArrayVar(p *[]string, name string, value []string, usage string) {
	if len(value) > 0 {
		*p = value[:]
	}
	fs.Var(&stringArrayFlag{dest: p, defaults: true}, name, usage)
}

// GoFlag wraps a flag.Flag object from the go standard library
// and implements the Flag interface above.
type GoFlag struct {
	*flag.Flag
}

func (gf *GoFlag) Name() string {
	return gf.Flag.Name
}

func (gf *GoFlag) Set(value string) error {
	err := gf.Flag.Value.Set(value)
	if err != nil {
		return err
	}
	gf.Flag.DefValue = value
	return nil
}
func (gf *GoFlag) SetContent(name string, data []byte) error {
	result, err := SetContent(gf.Flag.Value, name, data)
	if err != nil {
		return err
	}
	gf.Flag.DefValue = result
	return nil
}

// Command represents a command line command.
type Command interface {
	Name() string
	Hide(bool)
}

type CommandDefinition struct {
	Name    string
	Use     string
	Short   string
	Long    string
	Example string

	Aliases []string
}

type FlagDefinition struct {
	Name    string
	Help    string
	Default string
}

type FlagArg struct {
	*FlagDefinition
	flag.Value
}

type CommandAction func(flags []FlagArg, args []string) error

// Commander is a Command that is capable of having subcommands.
type Commander interface {
	Command
	AddCommand(def CommandDefinition, fl []FlagDefinition, action CommandAction) error
}

// An Augmenter is an object capable of providing default flag values, disable, add
// or modify sub commands of a generic CLI.
//
// Typically, it is invoked by a library that iterates over the flags of a command,
// and the existing commands defined.
//
// VisitFlag is invoked for each flag, with the method implementation allowed to call
// arbitrary methods on the flag.
//
// VisitCommands is invoked for each sub-command, with the method implementation allowed
// to call arbitrary methods on the command.
//
// At the end of the walk, Done is called.
//
// The user of Augmenter must assume that any of the pointers passed to the agumenter
// may be used until the point that Done() is invoked.
//
// Some resolvers may, for example, accumulate all the required flags to determine
// the value to lookup in a database with a single query.
//
// Concurrent access to the flag or command by the resolver is not allowed. The
// resolver must ensure that access to a given flag object is serialized.
type Augmenter interface {
	// VisitCommand is used to ask the Augmenter to configure a command.
	//
	// namespace is a string that identifies the parent command this command is defined on.
	// It is generally a string like "enkit.astore" identifying the "astore" subcommand of "enkit".
	//
	// Note that the caller will immediately call VisitFlag and other VisitCommand after this
	// command returns, without waiting for Done().
	VisitCommand(namespace string, command Command) (bool, error)

	// VisitFlag is used to ask the Augmenter to configure a flag.
	//
	// namespace is a string that identifies the command the flag is defined on.
	// It is generally a string like "enkit.astore" identifying the "astore" subcommand of "enkit".
	VisitFlag(namespace string, flag Flag) (bool, error)

	// Waits for all the visit details to be filled in.
	//
	// After Done() is invoked, the caller can assume that the flags and commands will no longer
	// be touched by the augmenter.
	Done() error
}

// SetContent is a utility function Augmenters can use to set the value of a flag.
//
// Let's say you have a flag that takes the path of a file, to load it. At run
// time, the value of the flag is the content of the file, rather than its path.
//
// SetContent will check to see if the flag implements the ContentValue interface,
// and set the content as necessary.
//
// The first string returned provides a suitable default value to show the user.
func SetContent(fl flag.Value, name string, value []byte) (string, error) {
	content, ok := fl.(ContentValue)
	if ok {
		return "<content>", content.SetContent(name, value)
	}
	newv := strings.TrimSpace(string(value))
	return newv, fl.Set(newv)
}

// Populator is a function that given a set of Augmenters, it is capable of invoking
// them to assign default flag values.
type Populator func(resolvers ...Augmenter) error

// GoPopulator returns a Populator capable of walking all the flags in the specified
// set, and assign defaults to those flags.
func GoPopulator(set *flag.FlagSet) Populator {
	return func(resolvers ...Augmenter) error {
		return PopulateDefaults(set, resolvers...)
	}
}

// PopulateDefaults will provide the defaults of your flags.
//
// It should be called before `flag.Parse()` to provide defaults to your flags.
//
// If you don't use FlagSet explicitly, you can just pass flag.CommandLine.
//
// For example, an application reading flags from the environment may use:
//
//     var server = flag.String("server", "127.0.0.1", "server to connect to")
//     func main(...) {
//       kflags.PopulateDefaults(flag.CommandLine, kflags.NewEnvAugmenter())
//       [...]
//
//       flag.Parse()
//
func PopulateDefaults(set *flag.FlagSet, resolvers ...Augmenter) error {
	errors := []error{}
	namespace := filepath.Base(set.Name())

	user_set := map[string]struct{}{}
	set.Visit(func(fl *flag.Flag) {
		user_set[fl.Name] = struct{}{}
	})

	// One resolver can provide defaults for flags used by the next resolver.
	for _, r := range resolvers {
		set.VisitAll(func(fl *flag.Flag) {
			// Never override flags explicitly set by users.
			if _, found := user_set[fl.Name]; found {
				return
			}

			if _, err := r.VisitFlag(namespace, &GoFlag{fl}); err != nil {
				errors = append(errors, err)
			}
		})

		if err := r.Done(); r != nil {
			errors = append(errors, err)
		}
	}

	return multierror.New(errors)
}
