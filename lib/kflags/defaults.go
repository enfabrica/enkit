package kflags

import (
	"flag"
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

// GoFlagSet wraps a flag.FloagSet from the go standard library and completes the
// implementation of the FlagSet interface in this module.
type GoFlagSet struct {
	*flag.FlagSet
}

func (fs *GoFlagSet) ByteFileVar(p *[]byte, name string, defaultFile string, usage string, mods ...ByteFileModifier) {
	fs.Var(NewByteFileFlag(p, defaultFile, mods...), name, usage)
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

// A Resolver is an object capable of providing default values for a flag.
//
// Typically, it is invoked by a library that iterates over the flags of a command.
//
// Visit is invoked for each flag, with the Visit implementation allowed to call
// arbitrary methods on the flag.
//
// At the end of the walk, Done is called.
//
// The user of Resolver must assume that the flag pointers passed may be used up
// until the point that Done() is invoked.
//
// Some resolvers may, for example, accumulate all the required flags to determine
// the value to lookup in a database with a single query.
//
// Concurrent access to the flag by the resolver is not allowed. The resolver must
// ensure that access to a given flag pointer is serialized.
type Resolver interface {
	Visit(namespace string, flag Flag) (bool, error)
	Done() error
}

// SetContent is a utility function Resolvers can use to set the value of a flag.
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

// Populator is a function that given a set of resolvers, it is capable of invoking
// them to assign default flag values.
type Populator func(resolvers ...Resolver) error

// GoPopulator returns a Populator capable of walking all the flags in the specified
// set, and assign defaults to those flags.
func GoPopulator(set *flag.FlagSet) Populator {
	return func(resolvers ...Resolver) error {
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
//       kflags.PopulateDefaults(flag.CommandLine, kflags.NewEnvResolver())
//       [...]
//
//       flag.Parse()
//
func PopulateDefaults(set *flag.FlagSet, resolvers ...Resolver) error {
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

			if _, err := r.Visit(namespace, &GoFlag{fl}); err != nil {
				errors = append(errors, err)
			}
		})

		if err := r.Done(); r != nil {
			errors = append(errors, err)
		}
	}

	return multierror.New(errors)
}
