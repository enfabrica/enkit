package kcobra

import (
	"github.com/enfabrica/enkit/lib/kflags"
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

type options struct {
	ehandlers []kflags.ErrorHandler
	printer   kflags.Printer
	argv      []string
	runner    func() error
	helper    func(*cobra.Command, []string) bool
}

type Modifier func(*cobra.Command, *options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(c *cobra.Command, o *options) error {
	for _, m := range mods {
		if err := m(c, o); err != nil {
			return err
		}
	}
	return nil
}

func WithPrinter(log kflags.Printer) Modifier {
	return func(c *cobra.Command, o *options) error {
		o.printer = log
		return nil
	}
}

func WithErrorHandler(eh ...kflags.ErrorHandler) Modifier {
	return func(c *cobra.Command, o *options) error {
		o.ehandlers = append(o.ehandlers, eh...)
		return nil
	}
}

func WithArgs(argv []string) Modifier {
	return func(c *cobra.Command, o *options) error {
		o.argv = argv
		return nil
	}
}

func WithRunner(runner func() error) Modifier {
	return func(c *cobra.Command, o *options) error {
		o.runner = runner
		return nil
	}
}

func WithHelper(helper func(c *cobra.Command, args []string) bool) Modifier {
	return func(c *cobra.Command, o *options) error {
		o.helper = helper
		return nil
	}
}

func Run(root *cobra.Command, mods ...Modifier) {
	o := options{argv: os.Args}

	err := Modifiers(mods).Apply(root, &o)
	if o.printer != nil {
		LogFlags(root, (logger.Printer)(o.printer))
	}

	// Cobra expects argv without argv[0], without the path of the command.
	if len(o.argv) >= 1 {
		o.argv = o.argv[1:]
	}
	root.SetArgs(o.argv)

	if o.runner != nil {
		original := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			err := o.runner()
			if err != nil {
				return err
			}
			if original != nil {
				return original(cmd, args)
			}
			return nil
		}
	}

	if o.helper != nil {
		original := root.HelpFunc()
		root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
			if !o.helper(cmd, args) {
				return
			}
			original(cmd, args)
		})
	}

	if err == nil {
		err = root.Execute()
	}
	if err != nil {
		cmd, _, nerr := root.Find(o.argv)
		if nerr != nil {
			cmd = root
		}

		for _, eh := range o.ehandlers {
			err = eh(err)
		}

		var ue *kflags.UsageError
		if errors.As(err, &ue) {
			root.Println(cmd.UsageString())
		}
		exit := 1
		var se *kflags.StatusError
		if ok := errors.As(err, &se); ok {
			exit = se.Code
		}

		root.Printf("ERROR: %s\n", err)
		os.Exit(exit)
	}
}

type Runnable interface {
	Run() error
}

type Helper interface {
	Help(cmd *cobra.Command, args []string) bool
}

func Runner(root *cobra.Command, argv []string, eh ...kflags.ErrorHandler) (*FlagSet, kflags.Populator, kflags.Runner) {
	if argv == nil {
		argv = os.Args
	}

	runner := func(fs kflags.FlagSet, p kflags.Printer) {
		mods := Modifiers{WithArgs(argv), WithErrorHandler(eh...), WithPrinter(p)}
		runnable, ok := fs.(Runnable)
		if ok {
			mods = append(mods, WithRunner(runnable.Run))
		}
		helping, ok := fs.(Helper)
		if ok {
			mods = append(mods, WithHelper(helping.Help))
		}
		Run(root, mods...)
	}
	return &FlagSet{FlagSet: root.PersistentFlags()}, CobraPopulator(root, argv), runner
}
