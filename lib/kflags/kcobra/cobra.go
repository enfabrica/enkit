package kcobra

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/populator"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"errors"
	"fmt"
	"os"
)

type FlagSet struct {
	*pflag.FlagSet
}

func (fs *FlagSet) ByteFileVar(p *[]byte, name string, defaultFile string, usage string, mods ...kflags.ByteFileModifier) {
	fs.Var(kflags.NewByteFileFlag(p, defaultFile, mods...), name, usage)
}

// Wrap errors in a StatusError to indicate a different exit value to be
// returned for this program.
type StatusError struct {
	error
	Code int
}

func NewStatusError(code int, err error) *StatusError {
	return &StatusError{error: err, Code: code}
}

func NewStatusErrorf(code int, f string, args ...interface{}) *StatusError {
	return &StatusError{error: fmt.Errorf(f, args...), Code: code}
}

// Wrap errors in an UsageError to indicate that the problem has been caused
// by incorrect flags by the user, and as such, the help screen should be
// printed.
type UsageError struct {
	error
}

func NewUsageError(err error) *UsageError {
	return &UsageError{error: err}
}

func NewUsageErrorf(f string, args ...interface{}) *UsageError {
	return &UsageError{error: fmt.Errorf(f, args...)}
}

type Command interface {
	Execute() error
	UsageString() string

	Println(...interface{})
	Printf(string, ...interface{})
}

func LogFlags(command *cobra.Command, log logger.Printer) {
	log("Running: %s", os.Args)
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		name := "--" + flag.Name
		if flag.Shorthand != "" {
			name += " (-" + flag.Shorthand + ")"
		}
		changed := "[not changed by user]"
		if flag.Changed {
			changed = fmt.Sprintf("[changed by user - original '%s']", flag.DefValue)
		}
		log("- flag %s value '%s' %s", name, flag.Value, changed)
	})
}

func RunWithDefaults(root *cobra.Command, popstore **populator.Populator, log *logger.Logger) {
	populator := populator.New("enkit", CobraPopulator(root, os.Args))
	populator.Register(&FlagSet{FlagSet: root.Flags()}, "")
	logger, _ := populator.PopulateDefaults(root.Name())

	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		LogFlags(root, logger.Infof)
	}

	if popstore != nil {
		*popstore = populator
	}
	if log != nil {
		*log = logger
	}

	Run(root)
}

func Run(root *cobra.Command) {
	if err := root.Execute(); err != nil {
		var ue *UsageError
		if errors.As(err, &ue) {
			root.Println(root.UsageString())
		}
		exit := 1
		var se *StatusError
		if ok := errors.As(err, &se); ok {
			exit = se.Code
		}

		root.Printf("ERROR: %s\n", err)
		os.Exit(exit)
	}
}
